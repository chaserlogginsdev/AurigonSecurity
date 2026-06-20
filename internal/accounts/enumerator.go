package accounts

import (
    "encoding/json"
    "os/exec"
	"log"
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

func Enumerate() ([]LocalAccount, error) {
    // Get basic local user info
    cmd := exec.Command("powershell", "-Command", "Get-LocalUser | ConvertTo-Json")
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

    json.Unmarshal(out, &users)

    // Get admin group members
    adminCmd := exec.Command("powershell", "-Command", "Get-LocalGroupMember -Group Administrators | ConvertTo-Json")
    adminOut, _ := adminCmd.Output()

    var admins []struct {
        Name string `json:"Name"`
    }
    json.Unmarshal(adminOut, &admins)

    adminMap := map[string]bool{}
    for _, a := range admins {
        adminMap[a.Name] = true
    }

    // Build final list
    accounts := []LocalAccount{}
    for _, u := range users {
        accounts = append(accounts, LocalAccount{
            Username:    u.Name,
            SID:         u.SID,
            Enabled:     u.Enabled,
            Description: u.Description,
            LastLogon:   u.LastLogon,
            IsAdmin:     adminMap[u.Name],
        })
    }

    return accounts, nil
}
