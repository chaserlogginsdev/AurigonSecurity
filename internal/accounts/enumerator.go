package accounts

import (
	"encoding/json"
	"log"
	"os/exec"
	"strings"
)

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

	// Handle single group (object) vs multiple groups (array)
	raw := strings.TrimSpace(string(out))
	if len(raw) == 0 {
		return []LocalGroup{}, nil
	}

	// If single object returned, wrap in array
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