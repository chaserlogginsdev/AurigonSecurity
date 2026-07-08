package accounts

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// ── Account enumeration ────────────────────────────────────────────────────

func Disable(username string) {
	cmd := exec.Command("net", "user", username, "/active:no")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to disable account %s: %v (%s)\n", username, err, string(out))
		return
	}
	log.Printf("Account %s disabled successfully\n", username)
}

const psUserScript = `
$users = Get-LocalUser | Select-Object Name, Enabled, Description,
    @{Name='SID';Expression={$_.SID.Value}},
    @{Name='LastLogon';Expression={if($_.LastLogon){$_.LastLogon.ToString('o')}else{''}}}
$arr = @($users)
ConvertTo-Json $arr
`

const psAdminScript = `
$members = Get-LocalGroupMember -Group Administrators |
    Select-Object @{Name='Name';Expression={($_.Name -split '\\')[-1]}}
$arr = @($members)
ConvertTo-Json $arr
`

const psGroupScript = `
$groups = Get-LocalGroup | Select-Object Name, Description
$result = @()
foreach ($g in $groups) {
    try {
        $members = Get-LocalGroupMember -Group $g.Name |
            Select-Object -ExpandProperty Name |
            ForEach-Object { ($_ -split '\\')[-1] }
        $arr = @($members)
    } catch {
        $arr = @()
    }
    $result += [PSCustomObject]@{
        Name        = $g.Name
        Description = if($g.Description){$g.Description}else{''}
        Members     = $arr
    }
}
$result | ConvertTo-Json -Depth 3
`

// psInfoScript collects hostname, domain membership, IP addresses, and OS version.
// Uses only built-in cmdlets available in PowerShell 5.1+.
const psInfoScript = `
$cs = Get-WmiObject Win32_ComputerSystem
$os = Get-WmiObject Win32_OperatingSystem

$ips = Get-NetIPAddress -AddressFamily IPv4 |
    Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
    Select-Object -ExpandProperty IPAddress

$arr = @($ips)

[PSCustomObject]@{
    Hostname       = $env:COMPUTERNAME
    Domain         = if($cs.PartOfDomain){ $cs.Domain }else{ '' }
    IsDomainJoined = [bool]$cs.PartOfDomain
    IPAddresses    = $arr
    OSVersion      = $os.Caption.Trim()
} | ConvertTo-Json
`

func Enumerate() ([]LocalAccount, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psUserScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var users []struct {
		Name        string `json:"Name"`
		SID         string `json:"SID"`
		Enabled     bool   `json:"Enabled"`
		Description string `json:"Description"`
		LastLogon   string `json:"LastLogon"`
	}
	if err := json.Unmarshal(out, &users); err != nil {
		return nil, err
	}

	adminCmd := exec.Command("powershell", "-NoProfile", "-Command", psAdminScript)
	adminOut, err := adminCmd.Output()

	adminMap := map[string]bool{}
	if err == nil {
		var admins []struct {
			Name string `json:"Name"`
		}
		if jsonErr := json.Unmarshal(adminOut, &admins); jsonErr == nil {
			for _, a := range admins {
				adminMap[strings.TrimSpace(a.Name)] = true
			}
		}
	}

	result := make([]LocalAccount, 0, len(users))
	for _, u := range users {
		result = append(result, LocalAccount{
			Username:    u.Name,
			SID:         u.SID,
			Enabled:     u.Enabled,
			Description: u.Description,
			LastLogon:   u.LastLogon,
			IsAdmin:     adminMap[u.Name],
		})
	}
	return result, nil
}

func EnumerateGroups() ([]LocalGroup, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psGroupScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(out))
	if len(raw) == 0 {
		return []LocalGroup{}, nil
	}

	if raw[0] == '{' {
		raw = "[" + raw + "]"
	}

	var groups []struct {
		Name        string   `json:"Name"`
		Description string   `json:"Description"`
		Members     []string `json:"Members"`
	}
	if err := json.Unmarshal([]byte(raw), &groups); err != nil {
		return nil, err
	}

	result := make([]LocalGroup, 0, len(groups))
	for _, g := range groups {
		members := g.Members
		if members == nil {
			members = []string{}
		}
		result = append(result, LocalGroup{
			Name:        g.Name,
			Description: g.Description,
			Members:     members,
		})
	}
	return result, nil
}

// EnumerateMachineInfo collects hostname, domain membership, IP addresses,
// and OS version from the local machine.
func EnumerateMachineInfo() (*MachineInfo, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psInfoScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var raw struct {
		Hostname       string   `json:"Hostname"`
		Domain         string   `json:"Domain"`
		IsDomainJoined bool     `json:"IsDomainJoined"`
		IPAddresses    []string `json:"IPAddresses"`
		OSVersion      string   `json:"OSVersion"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	ips := raw.IPAddresses
	if ips == nil {
		ips = []string{}
	}

	return &MachineInfo{
		Hostname:       raw.Hostname,
		Domain:         raw.Domain,
		IsDomainJoined: raw.IsDomainJoined,
		IPAddresses:    ips,
		OSVersion:      raw.OSVersion,
	}, nil
}

// ── Account management actions ──────────────────────────────────────────
// These use PowerShell cmdlets (New-LocalUser, Remove-LocalUser,
// Add/Remove-LocalGroupMember) rather than `net user` because they give
// cleaner error messages and support secure password handling.
//
// Passwords are passed via environment variables rather than command-line
// arguments so they don't appear in process listings (e.g. Task Manager,
// `Get-Process | Select CommandLine`). This is a meaningful improvement
// over passing secrets as CLI args, though not as strong as a dedicated
// secrets channel — worth revisiting if/when the agent adds a local IPC API.

const psCreateUserScript = `
$ErrorActionPreference = 'Stop'
$username = $env:AURIGON_ACTION_USERNAME
$password = $env:AURIGON_ACTION_PASSWORD
$isAdmin  = $env:AURIGON_ACTION_ISADMIN

$secure = ConvertTo-SecureString $password -AsPlainText -Force
New-LocalUser -Name $username -Password $secure -PasswordNeverExpires:$false | Out-Null

if ($isAdmin -eq 'true') {
    Add-LocalGroupMember -Group 'Administrators' -Member $username
}
Write-Output 'OK'
`

const psDeleteUserScript = `
$ErrorActionPreference = 'Stop'
$username = $env:AURIGON_ACTION_USERNAME
Remove-LocalUser -Name $username
Write-Output 'OK'
`

const psSetAdminScript = `
$ErrorActionPreference = 'Stop'
$username = $env:AURIGON_ACTION_USERNAME
$isAdmin  = $env:AURIGON_ACTION_ISADMIN

if ($isAdmin -eq 'true') {
    Add-LocalGroupMember -Group 'Administrators' -Member $username
} else {
    Remove-LocalGroupMember -Group 'Administrators' -Member $username
}
Write-Output 'OK'
`

// CreateAccount creates a new local user account, optionally adding it
// to the local Administrators group.
func CreateAccount(username, password string, isAdmin bool) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCreateUserScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+username,
		"AURIGON_ACTION_PASSWORD="+password,
		"AURIGON_ACTION_ISADMIN="+boolToStr(isAdmin),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create account: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

// DeleteAccount permanently removes a local user account.
func DeleteAccount(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psDeleteUserScript)
	cmd.Env = append(os.Environ(), "AURIGON_ACTION_USERNAME="+username)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete account: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

// SetAdminPrivilege adds or removes a user from the local Administrators group.
func SetAdminPrivilege(username string, isAdmin bool) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psSetAdminScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+username,
		"AURIGON_ACTION_ISADMIN="+boolToStr(isAdmin),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update admin privilege: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// cleanPSOutput strips noisy PowerShell error formatting down to something
// readable for the audit log / dashboard result field.
func cleanPSOutput(out []byte) string {
	s := strings.TrimSpace(string(out))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}