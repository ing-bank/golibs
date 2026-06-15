package kubemock

// DryRunOption is a helper function that sets the appropriate Kubernetes dry run option. Example:
//
//	metav1.CreateOptions{
//	  DryRun: kube.DryRunOption(isDryRun),
//	})
func DryRunOption(isDryRun bool) []string {
	if !isDryRun {
		return nil
	}
	return []string{"All"}
}
