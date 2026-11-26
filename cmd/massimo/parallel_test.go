// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package main

// The command line's tests run in parallel (AP TestSpeed, Block 8,
// D11-A): every test calls t.Parallel() before anything else, but those
// on serialTests, which Go runs one after the other before the parallel
// ones begin; a test that swaps a package variable says so through
// testkit.Swapping. docs/testing.md, "Parallel tests".

import (
	"testing"

	"go.metaworx.ch/massimo/internal/testkit"
)

// serialTests are the tests that do not run in parallel, and why. The
// command line keeps its options in package variables (the selected
// Cube, the log level, the update's hooks) and reads its settings from
// the process environment, so a test that sets either changes what
// every test beside it would see: making those parallel takes the
// options out of the package, which is the product's change, not the
// tests'.
var serialTests = map[string]string{
	// The settings in the process environment (t.Setenv).
	"TestConfigHidesASecretInAnyCase":       "unsets MASSIMO_API_TOKEN (unsetForTest), sets MASSIMO_API_Token",
	"TestConfigKnowsANameInAnyCase":         "unsets MASSIMO_LISTEN and MASSIMO_CUBE_KEY (unsetForTest), sets them in lower case",
	"TestConfigPrintsTLS":                   "sets the TLS variables",
	"TestConfigRedactsASecretInAnyCase":     "unsets MASSIMO_API_TOKEN (unsetForTest), sets MASSIMO_API_Token and a password",
	"TestConfigTiesThePlainLines":           "clears the Cube variables, sets MASSIMO_DATA and the plain Cube's variables, swaps plainLookup",
	"TestConfigViewHidesSecrets":            "sets MASSIMO_API_TOKEN, a Cube's key and MASSIMO_LISTEN",
	"TestDataDir":                           "sets MASSIMO_DATA",
	"TestNoEnv":                             "sets MASSIMO_ENV_FILE, as -no-env does",
	"TestReloadAppliesTheRebootSwitch":      "unsets settings variables (unsetForTest)",
	"TestReloadWarnsOfARefusedSetting":      "unsets settings variables (unsetForTest), clears the Cube variables, sets MASSIMO_HSTS and reloads a settings file into the environment",
	"TestReloadWarnsOfAnUnreadableFile":     "unsets settings variables (unsetForTest) and reloads a settings file into the environment",
	"TestReloadWarnsAsTheBoxChanges":        "clears the Cube variables, unsets the TLS variables (unsetForTest) and reloads a settings file into the environment",
	"TestReloadSeesANameInAnyCase":          "unsets settings variables (unsetForTest) and reloads a settings file into the environment",
	"TestReloadTellsTheNotes":               "unsets settings variables (unsetForTest) and reloads a settings file into the environment",
	"TestRestartHelper":                     "sets ProgramData",
	"TestSettingsReloader":                  "unsets settings variables (unsetForTest) and reloads a settings file into the environment",
	"TestSettingsReloaderDropsKeepOpen":     "unsets settings variables (unsetForTest) and reloads a settings file into the environment",
	"TestStartChecksCoverTheStart":          "clears the Cube variables and sets each case's settings in the environment, where the check of the Cubes reads them (serve.FileCubes); its runs of the built massimo are parallel subtests",
	"TestTLSCommand":                        "sets MASSIMO_DATA and the TLS variables",
	"TestTLSShowIsConfigsView":              "sets MASSIMO_DATA and the TLS variables",
	"TestHelpCreatesNoStore":                "changes the working directory (t.Chdir), swaps stdin and stdout (runUserWith)",
	"TestUserStoreDir":                      "changes the working directory (t.Chdir), swaps stderr, stdin and stdout",
	"TestUserCommands":                      "sets MASSIMO_DATA, swaps stdin and stdout (runUserWith)",
	"TestUserCommandsLog":                   "sets MASSIMO_DATA, swaps stdin and stdout (runUserWith)",
	"TestUserAddSaysWhatWasSetUp":           "sets MASSIMO_DATA, swaps stdin and stdout (runUserWith)",
	"TestUserListChain":                     "sets MASSIMO_DATA, swaps stdin and stdout (runUserWith)",
	"TestUserPasswordRule":                  "sets MASSIMO_DATA, swaps stdin and stdout (runUserWith)",
	"TestUserPasswordNames":                 "sets MASSIMO_DATA, swaps stdin and stdout (runUserWith)",
	"TestUserRename":                        "sets MASSIMO_DATA, swaps stdin and stdout (runUserWith)",
	"TestUserTokens":                        "sets MASSIMO_DATA, swaps stdin and stdout (runUserWith)",
	"TestStoreDowngradeCommand":             "sets MASSIMO_DATA, swaps serviceStateDir (raisedStore)",
	"TestRollbackStoreByTheKeptBinary":      "sets MASSIMO_DATA and the run-main switch, swaps serviceStateDir and stopService",
	"TestRollbackStoreByTheStoresDownParts": "sets MASSIMO_DATA and the run-main switch, swaps serviceStateDir and stopService",
	"TestRollbackStoreRunsAsTheStoresOwner": "sets MASSIMO_DATA and the run-main switch, swaps serviceStateDir and stopService",
	"TestRollbackStoreWithoutStore":         "sets MASSIMO_DATA and the run-main switch, swaps stopService",

	// The options kept in package variables.
	"TestDirectAndNoDirect":                        "sets proxyMode and the selected Cube, and Cube variables",
	"TestGlobalOptionsSelectTheCube":               "sets the selected Cube and showHidden, and Cube variables",
	"TestNetconfigAgainstTheFakeCube":              "sets the selected Cube (runNetconfigTest), as -sn does",
	"TestNetconfigBroadcastRecovery":               "sets the selected Cube (runNetconfigTest), as -sn does",
	"TestNetconfigReboot":                          "sets the selected Cube (runNetconfigTest), as -sn does",
	"TestNetconfigRefusesAGatewayOutsideTheSubnet": "sets the selected Cube (runNetconfigTest), as -sn does",
	"TestGlobalLogLevel":                           "sets logLevel and MASSIMO_LOG_LEVEL (resetLogLevel)",
	"TestNewLoggerLevel":                           "sets logLevel and MASSIMO_LOG_LEVEL (resetLogLevel)",
	"TestReloadListsTheLogLevel":                   "sets logLevel and MASSIMO_LOG_LEVEL (resetLogLevel)",
	"TestServeLoggerFormats":                       "sets logLevel and MASSIMO_LOG_LEVEL (resetLogLevel)",
	"TestServeRefusesAStrayArgument":               "sets logLevel and MASSIMO_LOG_LEVEL, clears the Cube variables",
	"TestServeRefusesAnUnknownLogLevel":            "sets logLevel and MASSIMO_LOG_LEVEL, clears the Cube variables",
	"TestConfigLinesNameTheDefault":                "sets serviceDefaults and MASSIMO_DATA",
	"TestDataDirOrder":                             "sets serviceDefaults and MASSIMO_DATA",
	"TestNoServiceUserOnWindows":                   "swaps the service's paths (serviceFiles)",
	"TestServiceUserDefaults":                      "swaps euid and the service's paths (serviceFiles)",
	"TestServiceEndWithoutStop":                    "swaps theService, sets ProgramData (tempServiceLog)",
	"TestServiceStopWaitsForServe":                 "swaps theService, sets ProgramData (tempServiceLog)",
	"TestServiceStopClosesTheStore":                "swaps theService, sets the Cube's address",
	"TestUpdateHelperListenKeepsTheStart":          "swaps theService, sets ProgramData (tempServiceLog) and MASSIMO_UPDATE_API",
	"TestRebootRequester":                          "sets rebootTake and rebootPoll",
	"TestWindowsRebooter":                          "swaps runCommand, sets SystemRoot",
	"TestHealthClientOwnAddresses":                 "swaps interfaceAddrs",

	// The update's hooks.
	"TestAptUpdateBusy":                  "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateFailures":              "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateFetchesTheRollingKey":  "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateHealthFails":           "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateNeedsTheSource":        "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateNothingNewer":          "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateOnlyAsTheHelper":       "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateRunsTheFixedCommands":  "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateSwitchFailures":        "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateSwitchesTheSuite":      "swaps aptRun and the helper's hooks (aptHelper)",
	"TestAptUpdateWaitsForTheListsLock":  "swaps aptRun and the helper's hooks (aptHelper)",
	"TestBrewUpdate":                     "swaps brewRun and the helper's hooks (brewHelper)",
	"TestBrewUpdateFailures":             "swaps brewRun and the helper's hooks (brewHelper)",
	"TestBrewUpdateRespectsNoAutoUpdate": "swaps brewRun and the helper's hooks (brewHelper), sets HOMEBREW_NO_AUTO_UPDATE",
	"TestHelperFailuresReachTheStatus":   "sets version, MASSIMO_UPDATE_CHANNEL",
	"TestUpdateChannelSources":           "sets flavour, MASSIMO_DATA and MASSIMO_UPDATE_CHANNEL",
	"TestUpdateRefusesPackaged":          "swaps packaged",

	// A signal to the whole process.
	"TestCatchSighup": "sends SIGHUP to the test's own process",
}

// Every test of the package calls t.Parallel() as its first statement,
// or is on serialTests with its reason and does not call it
// (testkit.ParallelOrListed).
func TestParallelOrListed(t *testing.T) {
	t.Parallel()
	testkit.ParallelOrListed(t, ".", serialTests)
}
