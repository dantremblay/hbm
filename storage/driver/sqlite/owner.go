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
		ContainerID: containerid,
		User:        user,
	}
	c.DB.Model(&ContainerOwner{}).Create(&co)
	if len(name) > 1 {
		con := ContainerOwner{
			ContainerID: fmt.Sprintf("name:%s", name),
			User:        user,
		}
		c.DB.Model(&ContainerOwner{}).Create(&con)
	}

	return nil
}

func (c *Config) RemoveContainerOwner(containerid string) {
	var toDelete []ContainerOwner

	collect := func(rows []ContainerOwner) {
		toDelete = append(toDelete, rows...)
	}

	// Try exact match on container_id (full ID)
	var exact []ContainerOwner
	c.DB.Where("container_id = ?", containerid).Find(&exact)
	collect(exact)

	// Try name:<containerid> match (user passed a name)
	name := fmt.Sprintf("name:%s", containerid)
	var byName []ContainerOwner
	c.DB.Where("container_id = ?", name).Find(&byName)
	collect(byName)

	// Try prefix match (short container ID)
	if len(exact) == 0 && len(byName) == 0 {
		prefix := fmt.Sprintf("%s%%", containerid)
		var byPrefix []ContainerOwner
		c.DB.Where("container_id LIKE ? AND container_id NOT LIKE 'name:%%'", prefix).Find(&byPrefix)
		collect(byPrefix)
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
			UserID:      row.UserID,
			Username:    user.Name,
			ContainerID: row.ContainerID,
			Model:       Model{CreatedAt: row.CreatedAt},
			RemovedAt:   now,
		})
		ids = append(ids, row.ID)
	}
	c.DB.Where("id IN (?)", ids).Delete(&ContainerOwner{})
}

func (c *Config) ListContainerOwnerIDs() []string {
	var rows []ContainerOwner
	c.DB.Where("container_id NOT LIKE 'name:%'").Find(&rows)
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ContainerID
	}
	return ids
}

func (c *Config) RemoveOrphanedContainerOwnerNames() int {
	// For each user, count ID rows and name: rows. If there are more name: rows
	// than ID rows, the oldest excess name: rows are orphans.
	var users []struct {
		UserID uint
	}
	c.DB.Model(&ContainerOwner{}).Select("DISTINCT user_id").Scan(&users)

	now := time.Now()
	var total int
	for _, u := range users {
		var idCount int
		c.DB.Model(&ContainerOwner{}).Where("user_id = ? AND container_id NOT LIKE 'name:%'", u.UserID).Count(&idCount)

		var nameCount int
		c.DB.Model(&ContainerOwner{}).Where("user_id = ? AND container_id LIKE 'name:%'", u.UserID).Count(&nameCount)

		excess := nameCount - idCount
		if excess <= 0 {
			continue
		}

		// Remove the oldest excess name: rows (those without a matching ID row)
		var orphans []ContainerOwner
		c.DB.Where("user_id = ? AND container_id LIKE 'name:%'", u.UserID).Order("id asc").Limit(excess).Find(&orphans)

		var ids []uint
		for _, row := range orphans {
			var user User
			c.DB.First(&user, row.UserID)

			c.DB.Create(&ContainerOwnerHistory{
				UserID:      row.UserID,
				Username:    user.Name,
				ContainerID: row.ContainerID,
				Model:       Model{CreatedAt: row.CreatedAt},
				RemovedAt:   now,
			})
			ids = append(ids, row.ID)
		}
		if len(ids) > 0 {
			c.DB.Where("id IN (?)", ids).Delete(&ContainerOwner{})
			total += len(ids)
		}
	}
	return total
}

func (c *Config) IsContainerOwner(username, containerid string) bool {
	var co ContainerOwner
	var u User
	var cnt int

	c.DB.Where("name = ?", username).First(&u)
	if u.ID == 0 {
		return false
	}

	name := fmt.Sprintf("name:%s", containerid)
	c.DB.Model(&co).Where("container_id = ? AND user_id = ?", name, u.ID).Count(&cnt)
	if cnt == 1 {
		return true
	}
	c.DB.Model(&co).Where("container_id = ? AND user_id = ?", containerid, u.ID).Count(&cnt)
	if cnt == 1 {
		return true
	}
	prefix := fmt.Sprintf("%s%%", containerid)
	prfm := false
	var cop []ContainerOwner
	c.DB.Where("container_id LIKE ?", prefix).Find(&cop)
	for _, p := range cop {
		if p.UserID != u.ID {
			return false
		}
		prfm = true
	}
	if prfm {
		return true
	}
	return false
}
