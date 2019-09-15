package helm

import (
	"dev/ecosia_intro/scripts/process"
	"fmt"
)

// New initializes the configuration for the Helm3 binary and for the service installation
func New(kubeconfig, buildDir, namespace, binName string) Helm {
	return Helm{
		Kubeconfig:  kubeconfig,
		ChartFolder: fmt.Sprintf("%s/binaries/helm", buildDir),
		Namespace:   namespace,
		InstallName: fmt.Sprintf("%s-%s", namespace, binName),
		Binary:      fmt.Sprintf("%s/binaries/helm", buildDir),
	}
}

// Helm holds the helm binary path and the installation parameters for a given chart
type Helm struct {
	Kubeconfig  string
	ChartFolder string
	Namespace   string
	InstallName string
	Binary      string
}

// Deploy runs a blue-green deployment on the defined helm configuration
func (h Helm) Deploy() error {
	err := h.ensureNamespace()
	if err != nil {
		return err
	}
	colour, version, err := h.getInstalledVersion()
	fmt.Println(colour, version)

	// --set autoscalingGroups[0].name=my-asg-name --set autoscalingGroups[0].maxSize=50 --set autoscalingGroups[0].minSize=2

	// Install service via helm binary (version v3.0.0-beta.3)
	// From version v3 onwards helm does not require tiller in cluster anymore
	return process.RunEnv(
		"HELM3",
		map[string]string{
			"KUBECONFIG": h.Kubeconfig,
		},
		[]string{h.Binary,
			"upgrade", h.InstallName,
			h.ChartFolder,
			"--install",
			"--namespace", h.Namespace,
		},
	)
}

func (h Helm) ensureNamespace() error {
	kcEnv := map[string]string{"KUBECONFIG": h.Kubeconfig}
	err := process.RunEnv(
		"HELM3", kcEnv, []string{"kubectl", "get", "namespace", h.Namespace},
	)

	if err != nil {
		e := process.RunEnv(
			"HELM3", kcEnv, []string{"kubectl", "create", "namespace", h.Namespace},
		)
		if e != nil {
			return fmt.Errorf(
				"failed to create namespace \"%s\" with kubeconfig \"%s\" - error: \"%s\"",
				h.Namespace, h.Kubeconfig, e)
		}
	}
	return nil
}

func (h Helm) getInstalledVersion() (
	string, string, error,
) {
	kcEnv := map[string]string{"KUBECONFIG": h.Kubeconfig}
	res, err := process.RunEnvRes(kcEnv,
		[]string{h.Binary, "get", "values", h.InstallName, "-n", h.Namespace})

	fmt.Println(err, res)
	return "", "", nil
}
