package steps

var registry = map[string]Handler{
	"disable-base-resource-prune": &DisableResourcePrune{},
	"generate-app-chart":     &GenerateAppChart{},
	"disable-sync-prune":     &DisableSyncPrune{},
	"swap-chart":             &SwapChart{},
	"enable-sync-prune":      &EnableSyncPrune{},
	"cleanup-common":         &CleanupCommon{},
	"update-deploy-workflow": &UpdateDeployWorkflow{},
}

// Lookup returns the handler for a step type, or false if not found.
func Lookup(stepType string) (Handler, bool) {
	h, ok := registry[stepType]
	return h, ok
}
