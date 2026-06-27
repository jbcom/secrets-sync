package driver

import "fmt"

var (
	ErrPathRequired = fmt.Errorf("path is required")

	// DriverNames is the set of built-in driver names. Drivers may also be
	// added at runtime via the Default registry; DriverIsSupported consults both.
	DriverNames = []DriverName{
		DriverNameAws,
		DriverNameVault,
		DriverNameIdentityCenter,
		DriverNameAzure,
		DriverNameGCP,
		DriverNameKubernetes,
		DriverNameHTTP,
	}
)

type DriverName string

const (
	DriverNameAws            DriverName = "aws"
	DriverNameVault          DriverName = "vault"
	DriverNameIdentityCenter DriverName = "awsIdentityCenter"
	DriverNameAzure          DriverName = "azure"
	DriverNameGCP            DriverName = "gcp"
	DriverNameKubernetes     DriverName = "kubernetes"
	DriverNameHTTP           DriverName = "http"
)

// DriverIsSupported reports whether a driver is recognized, either as a
// built-in name or as a backend registered in the Default registry.
func DriverIsSupported(driver DriverName) bool {
	for _, d := range DriverNames {
		if d == driver {
			return true
		}
	}
	return Default.SupportsSource(driver) || Default.SupportsTarget(driver)
}
