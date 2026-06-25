package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type GroupMember struct {
	Username string `json:"username"`
}

type GroupRow struct {
	ID          int      `json:"id"`
	MachineID   string   `json:"machine_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

type GroupInventoryRequest struct {
	DeviceID string `json:"device_id"`
	Groups   []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Members     []string `json:"members"`
	} `json:"groups"`
}

// Agent posts group inventory here
func groupInventoryHandler(w http.ResponseWriter, r *http.Request) {
	var req GroupInventoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Delete existing groups for this machine and re-insert
	db.Exec(`DELETE FROM groups WHERE machine_id = ?`, req.DeviceID)
	db.Exec(`DELETE FROM group_members WHERE machine_id = ?`, req.DeviceID)

	for _, g := range req.Groups {
		result, err := db.Exec(`
			INSERT INTO groups (machine_id, name, description)
			VALUES (?, ?, ?)
		`, req.DeviceID, g.Name, g.Description)
		if err != nil {
			continue
		}
		groupID, _ := result.LastInsertId()
		for _, member := range g.Members {
			if member == "" {
				continue
			}
			db.Exec(`
				INSERT INTO group_members (machine_id, group_id, username)
				VALUES (?, ?, ?)
			`, req.DeviceID, groupID, member)
		}
	}

	log.Printf("Groups updated: %s — %d groups\n", req.DeviceID, len(req.Groups))
	w.WriteHeader(http.StatusOK)
}

// Dashboard fetches groups for a machine
func groupsHandler(w http.ResponseWriter, r *http.Request) {
	machineID := r.URL.Query().Get("machine_id")
	if machineID == "" {
		http.Error(w, "machine_id required", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`
		SELECT id, machine_id, name, description
		FROM groups WHERE machine_id = ?
		ORDER BY name ASC
	`, machineID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	groups := []GroupRow{}
	for rows.Next() {
		var g GroupRow
		rows.Scan(&g.ID, &g.MachineID, &g.Name, &g.Description)

		// Fetch members for this group
		memberRows, err := db.Query(`
			SELECT username FROM group_members
			WHERE machine_id = ? AND group_id = ?
			ORDER BY username ASC
		`, machineID, g.ID)
		if err == nil {
			defer memberRows.Close()
			for memberRows.Next() {
				var username string
				memberRows.Scan(&username)
				g.Members = append(g.Members, username)
			}
		}
		if g.Members == nil {
			g.Members = []string{}
		}
		groups = append(groups, g)
	}

	json.NewEncoder(w).Encode(groups)
}