// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package server

// The codes of the refusals of Cubes, their refresh and reboot, backups
// and restores, the reboot of the computer, the server's restart and
// reload, updates, the log and the server's facts (AP ErrorCodes,
// Block 3), declared as errcodes.go's are. Where one condition is met at
// several sites in words of their own, it is one code, and the sites
// that word it otherwise say their words by refuseSaying.

// A Cube's settings as its form sends them (cubeRequest.apply): the
// name, the address and the proxy's address and serial, each field as
// typed.
var (
	codeCubeNameBad             = errorCode("cube_name_bad", "name: at most %d bytes of text", "max")
	codeCubeAddressBad          = errorCode("cube_address_bad", "address %q: want host or host:port", "address")
	codeCubeAddressPortBad      = errorCode("cube_address_port_bad", "address %q: bad port", "address")
	codeCubeProxyAddressBad     = errorCode("cube_proxy_address_bad", "proxy_address %q: want host or host:port", "address")
	codeCubeProxyAddressPortBad = errorCode("cube_proxy_address_port_bad", "proxy_address %q: bad port", "address")
	codeCubeProxySerialBad      = errorCode("cube_proxy_serial_bad", "proxy_serial %q: letters and digits, at most 16", "serial")
)

// A Cube added, restored, changed or removed (cubes_api.go), and the
// store those, the automatic update's answer and the server's facts
// need.
var (
	codeServerNoStore      = errorCode("server_no_store", "this server has no store")
	codeCubeServedAlready  = errorCode("cube_served_already", "cube %s is served already", "cube")
	codeCubeRestoreKept    = errorCode("cube_restore_not_removed", "cube %s has a row that is not removed: add it without restore", "cube")
	codeCubeRestoreUnknown = errorCode("cube_restore_unknown", "restore %q: no removed row of cube %s", "restore", "cube")
	codeCubeNotStored      = errorCode("cube_not_stored", "cube %s is served for this run only (-addr or -sn), not from the store", "cube")
)

// A Cube's refresh and reboot asked for (cube_refresh.go,
// cube_reboot.go), and a change of its settings (storedCube): what the
// Cube is not yet, or is doing.
var (
	codeCubeSerialNotKnownYet  = errorCode("cube_serial_not_known_yet", "the Cube's serial is not known yet")
	codeCubeAddressNotKnownYet = errorCode("cube_address_not_known_yet", "the Cube's address is not known yet")
	codeCubeRebooting          = errorCode("cube_rebooting", "the Cube is rebooting: its greeting after the reboot reads it again")
	codeCubeRebootPending      = errorCode("cube_reboot_pending", "%d radiograms pending as the Cube last said: a reboot drops them", "pending")
)

// A Cube's backups (backups.go, backups_api.go).
var (
	codeBackupNoHistory = errorCode("backup_no_history", "this Cube has no row in the store, so no backup history")
	codeCubeNotServed   = errorCode("cube_not_served",
		"cube %s is not served, so it takes no backup and no restore; its stored backups can still be listed, downloaded, compared and deleted", "cube")
	codeBackupUnknown = errorCode("backup_unknown", "no backup %d of cube %s", "backup", "cube")
)

// A restore planned and sent (restore_api.go): the restore going, the
// plan, the radio, and the backup chosen, stored or a file.
var (
	codeRestoreRunning       = errorCode("restore_running", "a restore is already running")
	codeRestorePlanChanged   = errorCode("restore_plan_changed", "the plan has changed since it was made; plan again")
	codeRestoreDutyCycle     = errorCode("restore_duty_cycle", "radio duty cycle at %d %%, above %d %%; wait for it to fall", "duty", "max")
	codeRestoreBackupNoCube  = errorCode("restore_backup_no_cube", "the backup names no cube; this is cube %s", "cube")
	codeRestoreBackupOfOther = errorCode("restore_backup_other_cube", "the backup is from cube %s, this is cube %s", "from", "cube")
	codeRestoreFileTooLarge  = errorCode("restore_file_too_large", "backup larger than %d bytes", "max")
	codeRestoreFileEmpty     = errorCode("restore_file_empty", "the body must be a backup file, JSON or YAML")
)

// The reboot of the computer the server runs on (reboot_api.go), and
// the reasons it shares with the restart and the update: a reboot
// waiting, a restart under way, an update running.
var (
	codeRebootUnavailable = errorCode("reboot_unavailable", "this server does not reboot the computer it runs on")
	codeRebootSwitchedOff = errorCode("reboot_switched_off", rebootSwitchedOff)
	codeRebootNoHelper    = errorCode("reboot_no_helper", "no reboot helper installed: sudo massimo install -reboot-helper")
	codeRebootBegun       = errorCode("reboot_begun", rebootBegun)
	codeRebootWaiting     = errorCode("reboot_waiting", rebootWaiting)
	codeRebootNotWaiting  = errorCode("reboot_not_waiting", "no reboot is waiting")
	codeRebootOpen        = errorCode("reboot_open", openRebootRefused)
	codeRebootDelayBad    = errorCode("reboot_delay_bad", "delay: want 0 to %d seconds", "max")
	codeRestartUnderWay   = errorCode("restart_under_way", "a restart is under way")
	codeUpdateRunning     = errorCode("update_running", "an update is running")
)

// The server's restart and the settings file's reload (restart_api.go).
var (
	codeRestartUnavailable   = errorCode("restart_unavailable", "no service manager would start this server again")
	codeRestartOpen          = errorCode("restart_open", openRestartRefused)
	codeReloadNoSettingsFile = errorCode("reload_no_settings_file", "this server reads no settings file")
)

// Why updates cannot be started from the API on this box (updateBlocked):
// its set-up, in the server's words, or in those of the package manager
// and the helper's start, which name the box's own.
var (
	codeUpdateManaged     = errorCode("update_managed", "managed by a package manager")
	codeUpdateOpen        = errorCode("update_open", "set MASSIMO_API_TOKEN or create a user to start updates from the API")
	codeUpdateFromService = errorCode("update_from_service", "start updates from the installed service (massimo install)")
	codeUpdateNoHelper    = errorCode("update_no_helper", "no update helper installed (see packaging/README.md)")
	codeUpdateHelperOff   = errorCode("update_helper_off", "the update helper is off: sudo massimo install turns it on")
	codeUpdateUnstartable = errorCode("update_helper_unstartable", "this process cannot start an update helper that would outlive it")
	codeUpdateNoChannel   = errorCode("update_no_channel",
		"no update channel: set the setting update.channel or MASSIMO_UPDATE_CHANNEL (a build without the Makefile has no flavour)")
	codeUpdateDevBuild = errorCode("update_dev_build", "this is a development or modified build; update it by hand (massimo update -force)")
)

// Why an update is not started now (beginUpdate), and the answer to the
// automatic update's question (putUpdateAnswer).
var (
	codeUpdateHeld = errorCode("update_held",
		"%s is held back: the update helper found nothing to install for it, and no check since has found it in %s", "version", "source")
	codeUpdateNotInSource = errorCode("update_not_in_source", "%s is out, but %s has it not yet", "version", "source")
	codeUpdateNoRelease   = errorCode("update_no_release", "no release on channel %s", "channel")
	codeUpdateUpToDate    = errorCode("update_up_to_date", "already up to date (%s)", "version")
	codeUpdateNotOffered  = errorCode("update_not_offered", "%q is not the version offered (%s)", "version", "offered")
	codeUpdateNoneOffered = errorCode("update_none_offered", "%q is not the version offered (none)", "version")
	codeUpdateAutoOff     = errorCode("update_auto_off",
		"automatic updates are off (the setting %s): there is no night to install it in or to skip", "setting")
)

// The log read from a cursor (log.go), and the server's settings in
// effect (serverinfo_api.go).
var (
	codeLogAfterBeyond = errorCode("log_after_beyond", "after %d is beyond the log's newest line: reload the log", "after")
	codeConfigNotShown = errorCode("config_not_shown", "this server does not show its settings")
)
