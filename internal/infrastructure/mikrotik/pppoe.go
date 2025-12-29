package mikrotik

import (
	"fmt"
)

// CreatePPPoESecret creates a new PPPoE secret
func (c *Client) CreatePPPoESecret(username, password, profile, localAddress, remoteAddress string) (string, error) {
	cmd := []string{
		"/ppp/secret/add",
		"=name=" + username,
		"=password=" + password,
		"=service=pppoe",
	}

	if profile != "" {
		cmd = append(cmd, "=profile="+profile)
	}
	if localAddress != "" {
		cmd = append(cmd, "=local-address="+localAddress)
	}
	if remoteAddress != "" {
		cmd = append(cmd, "=remote-address="+remoteAddress)
	}

	r, err := c.RunArgs(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to create ppp secret: %w", err)
	}

	// return ID of created item
	return r.Done.Map["ret"], nil
}

// UpdatePPPoESecret updates an existing PPPoE secret
func (c *Client) UpdatePPPoESecret(id, username, password, profile, localAddress, remoteAddress string) error {
	cmd := []string{
		"/ppp/secret/set",
		"=.id=" + id,
	}

	// Only update fields if they are provided/changed?
	// For simplicity, we update all main fields.
	// Make sure we don't accidentally clear fields if empty string means "no change".
	// But usually in set command, we set what we want.

	if username != "" {
		cmd = append(cmd, "=name="+username)
	}
	if password != "" {
		cmd = append(cmd, "=password="+password)
	}
	if profile != "" {
		cmd = append(cmd, "=profile="+profile)
	}
	if localAddress != "" {
		cmd = append(cmd, "=local-address="+localAddress)
	}
	if remoteAddress != "" {
		cmd = append(cmd, "=remote-address="+remoteAddress)
	}

	_, err := c.RunArgs(cmd)
	if err != nil {
		return fmt.Errorf("failed to update ppp secret: %w", err)
	}
	return nil
}

// DeletePPPoESecret deletes a PPPoE secret by ID
func (c *Client) DeletePPPoESecret(id string) error {
	cmd := []string{
		"/ppp/secret/remove",
		"=.id=" + id,
	}
	_, err := c.RunArgs(cmd)
	if err != nil {
		return fmt.Errorf("failed to delete ppp secret: %w", err)
	}
	return nil
}

// FindPPPoESecretID returns the ID of a PPPoE secret by username
func (c *Client) FindPPPoESecretID(username string) (string, error) {
	cmd := []string{
		"/ppp/secret/print",
		"?name=" + username,
		"=.proplist=.id",
	}

	r, err := c.RunArgs(cmd)
	if err != nil {
		return "", err
	}

	if len(r.Re) == 0 {
		return "", nil // Not found
	}

	return r.Re[0].Map[".id"], nil
}
