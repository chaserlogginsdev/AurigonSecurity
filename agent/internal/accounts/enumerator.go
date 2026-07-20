package accounts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ── Account enumeration ────────────────────────────────────────────────────

const psUserScript = `
$users = Get-LocalUser | Select-Object Name, Enabled, Description, FullName,
    @{Name='SID';Expression={$_.SID.Value}},
    @{Name='LastLogon';Expression={if($_.LastLogon){$_.LastLogon.ToString('o')}else{''}}},
    @{Name='PasswordLastSet';Expression={if($_.PasswordLastSet){$_.PasswordLastSet.ToString('o')}else{''}}},
    @{Name='PasswordExpiresDate';Expression={if($_.PasswordExpires){$_.PasswordExpires.ToString('o')}else{''}}},
    @{Name='AccountExpiresDate';Expression={if($_.AccountExpires){$_.AccountExpires.ToString('o')}else{''}}},
    PasswordNeverExpires, PasswordRequired, UserMayChangePassword
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
$ErrorActionPreference = 'SilentlyContinue'

$cs   = Get-WmiObject Win32_ComputerSystem
$os   = Get-WmiObject Win32_OperatingSystem
$disk = Get-WmiObject Win32_LogicalDisk -Filter "DeviceID='C:'"

$ips = Get-NetIPAddress -AddressFamily IPv4 |
    Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
    Select-Object -ExpandProperty IPAddress
$ipArr = @($ips)

$bootTime    = $os.ConvertToDateTime($os.LastBootUpTime)
$uptimeHours = [math]::Round(((Get-Date) - $bootTime).TotalHours, 1)

$pendingReboot = $false
if (Test-Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending') { $pendingReboot = $true }
if (Test-Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired') { $pendingReboot = $true }

$defenderEnabled  = $false
$defenderRealtime = $false
try {
    $mp = Get-MpComputerStatus
    $defenderEnabled  = [bool]$mp.AMServiceEnabled
    $defenderRealtime = [bool]$mp.RealTimeProtectionEnabled
} catch {}

$policyMinLen  = 0
$policyLockout = 0
$policyMaxAge  = 0
$netAccounts = net accounts 2>$null
foreach ($line in $netAccounts) {
    if ($line -match 'Minimum password length\s*:\s*(\d+)')  { $policyMinLen  = [int]$matches[1] }
    if ($line -match 'Lockout threshold\s*:\s*(\d+)')         { $policyLockout = [int]$matches[1] }
    if ($line -match 'Maximum password age.*:\s*(\d+)')       { $policyMaxAge  = [int]$matches[1] }
}

$failedLogonCount = 0
try {
    $events = Get-WinEvent -FilterHashtable @{LogName='Security'; Id=4625; StartTime=(Get-Date).AddHours(-24)} -MaxEvents 200 -ErrorAction Stop
    $failedLogonCount = $events.Count
} catch { $failedLogonCount = 0 }

[PSCustomObject]@{
    Hostname                   = $env:COMPUTERNAME
    Domain                     = if($cs.PartOfDomain){ $cs.Domain }else{ '' }
    IsDomainJoined             = [bool]$cs.PartOfDomain
    IPAddresses                = $ipArr
    OSVersion                  = $os.Caption.Trim()
    OSBuild                    = "$($os.BuildNumber)"
    UptimeHours                = $uptimeHours
    FreeDiskGB                 = if($disk){ [math]::Round($disk.FreeSpace / 1GB, 1) } else { 0 }
    TotalMemoryGB              = [math]::Round($cs.TotalPhysicalMemory / 1GB, 1)
    PendingReboot              = $pendingReboot
    DefenderEnabled            = $defenderEnabled
    DefenderRealtimeProtection = $defenderRealtime
    PasswordMinLength          = $policyMinLen
    PasswordLockoutThreshold   = $policyLockout
    PasswordMaxAgeDays         = $policyMaxAge
    FailedLogonCount24h        = $failedLogonCount
} | ConvertTo-Json
`

func Enumerate() ([]LocalAccount, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psUserScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var users []struct {
		Name                string `json:"Name"`
		SID                 string `json:"SID"`
		Enabled             bool   `json:"Enabled"`
		Description         string `json:"Description"`
		FullName            string `json:"FullName"`
		LastLogon           string `json:"LastLogon"`
		PasswordLastSet     string `json:"PasswordLastSet"`
		PasswordExpiresDate string `json:"PasswordExpiresDate"`
		AccountExpiresDate  string `json:"AccountExpiresDate"`
		PasswordNeverExpires bool  `json:"PasswordNeverExpires"`
		PasswordRequired    bool   `json:"PasswordRequired"`
		UserMayChangePassword bool `json:"UserMayChangePassword"`
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
			Username:               u.Name,
			SID:                    u.SID,
			Enabled:                u.Enabled,
			Description:            u.Description,
			FullName:               u.FullName,
			LastLogon:              u.LastLogon,
			IsAdmin:                adminMap[u.Name],
			PasswordLastSet:        u.PasswordLastSet,
			PasswordExpiresDate:    u.PasswordExpiresDate,
			AccountExpiresDate:     u.AccountExpiresDate,
			PasswordNeverExpires:   u.PasswordNeverExpires,
			PasswordRequired:       u.PasswordRequired,
			UserMayChangePassword:  u.UserMayChangePassword,
			DaysSinceLastLogon:     daysSince(u.LastLogon),
			IsBuiltIn:              isBuiltInSID(u.SID),
		})
	}
	return result, nil
}

// daysSince returns the number of days since an ISO-8601 timestamp, or -1
// if the timestamp is empty (account has never logged on).
func daysSince(isoTime string) int {
	if isoTime == "" {
		return -1
	}
	t, err := time.Parse(time.RFC3339, isoTime)
	if err != nil {
		return -1
	}
	return int(time.Since(t).Hours() / 24)
}

// isBuiltInSID flags Windows' own default accounts (Administrator, Guest,
// DefaultAccount, WDAGUtilityAccount) so the dashboard can distinguish
// system accounts from real user-created ones. These use well-known
// relative IDs (RIDs) that are consistent across all Windows machines.
func isBuiltInSID(sid string) bool {
	builtInRIDs := []string{"-500", "-501", "-503", "-504"}
	for _, rid := range builtInRIDs {
		if strings.HasSuffix(sid, rid) {
			return true
		}
	}
	return false
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
		Hostname                   string   `json:"Hostname"`
		Domain                     string   `json:"Domain"`
		IsDomainJoined             bool     `json:"IsDomainJoined"`
		IPAddresses                []string `json:"IPAddresses"`
		OSVersion                  string   `json:"OSVersion"`
		OSBuild                    string   `json:"OSBuild"`
		UptimeHours                float64  `json:"UptimeHours"`
		FreeDiskGB                 float64  `json:"FreeDiskGB"`
		TotalMemoryGB              float64  `json:"TotalMemoryGB"`
		PendingReboot              bool     `json:"PendingReboot"`
		DefenderEnabled            bool     `json:"DefenderEnabled"`
		DefenderRealtimeProtection bool     `json:"DefenderRealtimeProtection"`
		PasswordMinLength          int      `json:"PasswordMinLength"`
		PasswordLockoutThreshold   int      `json:"PasswordLockoutThreshold"`
		PasswordMaxAgeDays         int      `json:"PasswordMaxAgeDays"`
		FailedLogonCount24h        int      `json:"FailedLogonCount24h"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	ips := raw.IPAddresses
	if ips == nil {
		ips = []string{}
	}

	return &MachineInfo{
		Hostname:                   raw.Hostname,
		Domain:                     raw.Domain,
		IsDomainJoined:             raw.IsDomainJoined,
		IPAddresses:                ips,
		OSVersion:                  raw.OSVersion,
		OSBuild:                    raw.OSBuild,
		UptimeHours:                raw.UptimeHours,
		FreeDiskGB:                 raw.FreeDiskGB,
		TotalMemoryGB:              raw.TotalMemoryGB,
		PendingReboot:              raw.PendingReboot,
		DefenderEnabled:            raw.DefenderEnabled,
		DefenderRealtimeProtection: raw.DefenderRealtimeProtection,
		PasswordMinLength:          raw.PasswordMinLength,
		PasswordLockoutThreshold:   raw.PasswordLockoutThreshold,
		PasswordMaxAgeDays:         raw.PasswordMaxAgeDays,
		FailedLogonCount24h:        raw.FailedLogonCount24h,
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

// ── Additional account management actions (AD-style operations) ────────

const psResetPasswordScript = `
$ErrorActionPreference = 'Stop'
$username = $env:AURIGON_ACTION_USERNAME
$password = $env:AURIGON_ACTION_PASSWORD
$secure = ConvertTo-SecureString $password -AsPlainText -Force
Set-LocalUser -Name $username -Password $secure
Write-Output 'OK'
`

const psUnlockAccountScript = `
$ErrorActionPreference = 'Stop'
$username = $env:AURIGON_ACTION_USERNAME
Unlock-LocalUser -Name $username
Write-Output 'OK'
`

// ResetPassword sets a new password for a local account.
func ResetPassword(username, newPassword string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if newPassword == "" {
		return fmt.Errorf("password is required")
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psResetPasswordScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+username,
		"AURIGON_ACTION_PASSWORD="+newPassword,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reset password: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

// RequirePasswordChangeAtNextLogon flags the account so the user must
// change their password the next time they sign in.
func RequirePasswordChangeAtNextLogon(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}

	cmd := exec.Command("net", "user", username, "/logonpasswordchg:yes")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set password-change flag: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

// UnlockAccount clears a local account lockout (e.g. after too many
// failed login attempts).
func UnlockAccount(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psUnlockAccountScript)
	cmd.Env = append(os.Environ(), "AURIGON_ACTION_USERNAME="+username)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to unlock account: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

// ── Extended account lifecycle actions ──────────────────────────────────

const psRenameScript = `
$ErrorActionPreference = 'Stop'
Rename-LocalUser -Name $env:AURIGON_ACTION_USERNAME -NewName $env:AURIGON_ACTION_NEWUSERNAME
Write-Output 'OK'
`

const psUpdateDetailsScript = `
$ErrorActionPreference = 'Stop'
Set-LocalUser -Name $env:AURIGON_ACTION_USERNAME -FullName $env:AURIGON_ACTION_FULLNAME -Description $env:AURIGON_ACTION_DESCRIPTION
Write-Output 'OK'
`

const psPasswordNeverExpiresScript = `
$ErrorActionPreference = 'Stop'
$never = [System.Convert]::ToBoolean($env:AURIGON_ACTION_NEVEREXPIRES)
Set-LocalUser -Name $env:AURIGON_ACTION_USERNAME -PasswordNeverExpires:$never
Write-Output 'OK'
`

const psAccountExpirationScript = `
$ErrorActionPreference = 'Stop'
$expires = $env:AURIGON_ACTION_EXPIRES
if ($expires -eq '' -or $expires -eq 'never') {
    Set-LocalUser -Name $env:AURIGON_ACTION_USERNAME -AccountExpires $null
} else {
    $date = [DateTime]::Parse($expires)
    Set-LocalUser -Name $env:AURIGON_ACTION_USERNAME -AccountExpires $date
}
Write-Output 'OK'
`

const psAddToGroupScript = `
$ErrorActionPreference = 'Stop'
Add-LocalGroupMember -Group $env:AURIGON_ACTION_GROUP -Member $env:AURIGON_ACTION_USERNAME
Write-Output 'OK'
`

const psRemoveFromGroupScript = `
$ErrorActionPreference = 'Stop'
Remove-LocalGroupMember -Group $env:AURIGON_ACTION_GROUP -Member $env:AURIGON_ACTION_USERNAME
Write-Output 'OK'
`

const psCreateGroupScript = `
$ErrorActionPreference = 'Stop'
New-LocalGroup -Name $env:AURIGON_ACTION_GROUP -Description $env:AURIGON_ACTION_DESCRIPTION
Write-Output 'OK'
`

const psDeleteGroupScript = `
$ErrorActionPreference = 'Stop'
Remove-LocalGroup -Name $env:AURIGON_ACTION_GROUP
Write-Output 'OK'
`

// psSessionsScript enumerates currently logged-on sessions via `query user`.
// Best-effort parsing: query user's column layout shifts slightly when a
// session has no SESSIONNAME (e.g. disconnected sessions), so this handles
// both the 5-column and 6-column cases.
const psSessionsScript = `
$ErrorActionPreference = 'SilentlyContinue'
$lines = query user 2>$null
$sessions = @()
if ($lines) {
    for ($i = 1; $i -lt $lines.Count; $i++) {
        $line = ($lines[$i] -replace '^>', ' ').Trim()
        if ($line -eq '') { continue }
        $parts = [regex]::Split($line, '\s{2,}')
        if ($parts.Count -ge 5) {
            $sessions += [PSCustomObject]@{
                Username    = $parts[0]
                SessionName = $parts[1]
                Id          = $parts[2]
                State       = $parts[3]
                IdleTime    = $parts[4]
                LogonTime   = if ($parts.Count -gt 5) { $parts[5] } else { '' }
            }
        } elseif ($parts.Count -eq 4) {
            $sessions += [PSCustomObject]@{
                Username    = $parts[0]
                SessionName = ''
                Id          = $parts[1]
                State       = $parts[2]
                IdleTime    = $parts[3]
                LogonTime   = ''
            }
        }
    }
}
$arr = @($sessions)
ConvertTo-Json $arr
`

func RenameAccount(oldUsername, newUsername string) error {
	if oldUsername == "" || newUsername == "" {
		return fmt.Errorf("both old and new username are required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psRenameScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+oldUsername,
		"AURIGON_ACTION_NEWUSERNAME="+newUsername,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rename account: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

func UpdateAccountDetails(username, fullName, description string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psUpdateDetailsScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+username,
		"AURIGON_ACTION_FULLNAME="+fullName,
		"AURIGON_ACTION_DESCRIPTION="+description,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update account details: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

func SetPasswordNeverExpires(username string, neverExpires bool) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psPasswordNeverExpiresScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+username,
		"AURIGON_ACTION_NEVEREXPIRES="+boolToStr(neverExpires),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update password expiration: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

// SetAccountExpiration sets an account expiration date. Pass "" or "never"
// to clear expiration. Otherwise pass a date string like "2026-12-31".
func SetAccountExpiration(username, expires string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psAccountExpirationScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+username,
		"AURIGON_ACTION_EXPIRES="+expires,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set account expiration: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

func AddToGroup(username, group string) error {
	if username == "" || group == "" {
		return fmt.Errorf("username and group are required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psAddToGroupScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+username,
		"AURIGON_ACTION_GROUP="+group,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add to group: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

func RemoveFromGroup(username, group string) error {
	if username == "" || group == "" {
		return fmt.Errorf("username and group are required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psRemoveFromGroupScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_USERNAME="+username,
		"AURIGON_ACTION_GROUP="+group,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove from group: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

func CreateGroup(name, description string) error {
	if name == "" {
		return fmt.Errorf("group name is required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psCreateGroupScript)
	cmd.Env = append(os.Environ(),
		"AURIGON_ACTION_GROUP="+name,
		"AURIGON_ACTION_DESCRIPTION="+description,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create group: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

func DeleteGroup(name string) error {
	if name == "" {
		return fmt.Errorf("group name is required")
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psDeleteGroupScript)
	cmd.Env = append(os.Environ(), "AURIGON_ACTION_GROUP="+name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete group: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

// ForceLogoff finds the active session for a username and logs it off.
func ForceLogoff(username string) error {
	sessions, err := EnumerateSessions()
	if err != nil {
		return fmt.Errorf("could not enumerate sessions: %w", err)
	}
	var sessionID string
	for _, s := range sessions {
		if strings.EqualFold(s.Username, username) {
			sessionID = s.ID
			break
		}
	}
	if sessionID == "" {
		return fmt.Errorf("no active session found for %s", username)
	}

	cmd := exec.Command("logoff", sessionID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to force logoff: %v (%s)", err, cleanPSOutput(out))
	}
	return nil
}

// EnumerateSessions returns currently logged-on user sessions.
func EnumerateSessions() ([]SessionInfo, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psSessionsScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return []SessionInfo{}, nil
	}
	if raw[0] == '{' {
		raw = "[" + raw + "]"
	}

	var sessions []struct {
		Username    string `json:"Username"`
		SessionName string `json:"SessionName"`
		Id          string `json:"Id"`
		State       string `json:"State"`
		IdleTime    string `json:"IdleTime"`
		LogonTime   string `json:"LogonTime"`
	}
	if err := json.Unmarshal([]byte(raw), &sessions); err != nil {
		return nil, err
	}

	result := make([]SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, SessionInfo{
			Username:    s.Username,
			SessionName: s.SessionName,
			ID:          s.Id,
			State:       s.State,
			IdleTime:    s.IdleTime,
			LogonTime:   s.LogonTime,
		})
	}
	return result, nil
}