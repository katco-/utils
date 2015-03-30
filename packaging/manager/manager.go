// Copyright 2015 Canonical Ltd.
// Copyright 2015 Cloudbase Solutions SRL
// Licensed under the AGPLv3, see LICENCE file for details.

package manager

import (
	"fmt"
	"strings"

	"github.com/juju/utils"
	"github.com/juju/utils/packaging/commands"
	"github.com/juju/utils/proxy"
)

// basePackageManager is the struct which executes various
// packaging-related operations.
type basePackageManager struct {
	cmder commands.PackageCommander
}

// InstallPrerequisite implements PackageManager.
func (pm *basePackageManager) InstallPrerequisite() error {
	_, _, err := runCommandWithRetry(pm.cmder.InstallPrerequisiteCmd())
	return err
}

// Update implements PackageManager.
func (pm *basePackageManager) Update() error {
	_, _, err := runCommandWithRetry(pm.cmder.UpdateCmd())
	return err
}

// Upgrade implements PackageManager.
func (pm *basePackageManager) Upgrade() error {
	_, _, err := runCommandWithRetry(pm.cmder.UpgradeCmd())
	return err
}

// Install implements PackageManager.
func (pm *basePackageManager) Install(packs ...string) error {
	_, _, err := runCommandWithRetry(pm.cmder.InstallCmd(packs...))
	return err
}

// Remove implements PackageManager.
func (pm *basePackageManager) Remove(packs ...string) error {
	_, _, err := runCommandWithRetry(pm.cmder.RemoveCmd(packs...))
	return err
}

// Purge implements PackageManager.
func (pm *basePackageManager) Purge(packs ...string) error {
	_, _, err := runCommandWithRetry(pm.cmder.PurgeCmd(packs...))
	return err
}

// IsInstalled implements PackageManager.
func (pm *basePackageManager) IsInstalled(pack string) bool {
	args := strings.Fields(pm.cmder.IsInstalledCmd(pack))

	_, err := utils.RunCommand(args[0], args[1:]...)
	return err == nil
}

// AddRepository implements PackageManager.
func (pm *basePackageManager) AddRepository(repo string) error {
	_, _, err := runCommandWithRetry(pm.cmder.AddRepositoryCmd(repo))
	return err
}

// RemoveRepository implements PackageManager.
func (pm *basePackageManager) RemoveRepository(repo string) error {
	_, _, err := runCommandWithRetry(pm.cmder.RemoveRepositoryCmd(repo))
	return err
}

// Cleanup implements PackageManager.
func (pm *basePackageManager) Cleanup() error {
	_, _, err := runCommandWithRetry(pm.cmder.CleanupCmd())
	return err
}

// SetProxy implements PackageManager.
func (pm *basePackageManager) SetProxy(settings proxy.Settings) error {
	cmds := pm.cmder.SetProxyCmds(settings)

	for _, cmd := range cmds {
		args := []string{"bash", "-c", fmt.Sprintf("%q", cmd)}
		out, err := runCommand(args[0], args[1:]...)
		if err != nil {
			logger.Errorf("command failed: %v\nargs: %#v\n%s", err, args, string(out))
			return fmt.Errorf("command failed: %v", err)
		}
	}

	return nil
}
