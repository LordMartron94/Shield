package internal

import (
	"echo"
	"essence"
)

const shieldSystemIDString = "69be6623-df9d-46bb-a4f1-063cf08c2c86"

var shieldSystemID essence.UUID

func init() {
	uuid, _ := echo.EchoSystemRegisterFromString(shieldSystemIDString, echo.EchoSystemConfiguration{
		MinLogLevel:    echo.TRACE,
		SystemPrefixes: []string{"Shield"},
	})

	shieldSystemID = uuid
}
