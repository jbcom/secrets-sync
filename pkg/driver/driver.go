package driver

import "fmt"

var (
	ErrPathRequired = fmt.Errorf("path is required")

	DriverNames = []DriverName{
		DriverNameAws,
		DriverNameVault,
		DriverNameIdentityCenter,
	}
)

type DriverName string

const (
	DriverNameAws            DriverName = "aws"
	DriverNameVault          DriverName = "vault"
	DriverNameIdentityCenter DriverName = "awsIdentityCenter"
)

func DriverIsSupported(driver DriverName) bool {
	for _, d := range DriverNames {
		if d == driver {
			return true
		}
	}
	return false
}
