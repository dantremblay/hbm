package sqlite

import (
	"fmt"
	"time"
)

func (c *Config) SetContainerOwner(username, name, containerid string) error {
	var user User

	c.DB.Where("name = ?", username).First(&user)
	if user.ID == 0 {
		return nil //FIXME
	}

	co := ContainerOwner{
		ContainerID:   containerid,
		ContainerName: name,
		User:          user,
	}
	c.DB.Model(&ContainerOwner{}).Create(&co)

	return nil
}

func (c *Config) RemoveContainerOwner(containerid string) {
	var toDelete []ContainerOwner

	// Try exact match on container_id
	var exact []ContainerOwner
	c.DB.Where("container_id = ?", containerid).Find(&exact)
	toDelete = append(toDelete, exact...)

	// Try match by container_name
	if len(exact) == 0 {
		var byName []ContainerOwner
		c.DB.Where("container_name = ?", containerid).Find(&byName)
		toDelete = append(toDelete, byName...)
	}

	// Try prefix match on container_id (short ID)
	if len(toDelete) == 0 {
		prefix := fmt.Sprintf("%s%%", containerid)
		var byPrefix []ContainerOwner
		c.DB.Where("container_id LIKE ?", prefix).Find(&byPrefix)
		toDelete = append(toDelete, byPrefix...)
	}

	if len(toDelete) == 0 {
		return
	}

	now := time.Now()
	var ids []uint
	for _, row := range toDelete {
		var user User
		c.DB.First(&user, row.UserID)

		c.DB.Create(&ContainerOwnerHistory{
			UserID:        row.UserID,
			Username:      user.Name,
			ContainerID:   row.ContainerID,
			ContainerName: row.ContainerName,
			Model:         Model{CreatedAt: row.CreatedAt},
			RemovedAt:     now,
		})
		ids = append(ids, row.ID)
	}
	c.DB.Where("id IN (?)", ids).Delete(&ContainerOwner{})
}

func (c *Config) ListContainerOwnerIDs() []string {
	var rows []ContainerOwner
	c.DB.Find(&rows)
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ContainerID
	}
	return ids
}

func (c *Config) IsContainerOwner(username, containerid string) bool {
	var u User
	var cnt int

	c.DB.Where("name = ?", username).First(&u)
	if u.ID == 0 {
		return false
	}

	// Check by container_name
	c.DB.Model(&ContainerOwner{}).Where("container_name = ? AND user_id = ?", containerid, u.ID).Count(&cnt)
	if cnt > 0 {
		return true
	}

	// Check by exact container_id
	c.DB.Model(&ContainerOwner{}).Where("container_id = ? AND user_id = ?", containerid, u.ID).Count(&cnt)
	if cnt > 0 {
		return true
	}

	// Check by container_id prefix (short ID)
	prefix := fmt.Sprintf("%s%%", containerid)
	var matches []ContainerOwner
	c.DB.Where("container_id LIKE ?", prefix).Find(&matches)
	for _, m := range matches {
		if m.UserID != u.ID {
			return false
		}
	}
	if len(matches) > 0 {
		return true
	}

	return false
}
