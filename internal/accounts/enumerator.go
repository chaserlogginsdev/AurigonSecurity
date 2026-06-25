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

// psUserScript wraps the result in an array explicitly so we always get
// a JSON array even on PowerShell 5 (which lacks ConvertTo-Json -AsArray).
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