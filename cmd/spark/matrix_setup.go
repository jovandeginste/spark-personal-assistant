package main

import (
	"fmt"

	"github.com/jovandeginste/spark-personal-assistant/pkg/ai"
	"github.com/jovandeginste/spark-personal-assistant/pkg/matrix"
	"github.com/jovandeginste/spark-personal-assistant/pkg/router"
)

func (c *cli) initMatrixClient(instanceName string, aiClient ai.Client, r *router.Router) (*matrix.MatrixConfig, error) {
	mc := &matrix.MatrixConfig{App: c.app, InstanceName: instanceName, Router: r}
	mc.AIClient = aiClient

	aiData, err := c.app.BuildData()
	if err != nil {
		return nil, err
	}
	mc.AIData = aiData

	if err := mc.InitClient(); err != nil {
		return nil, err
	}
	mc.ConfigureSyncer()

	if err := mc.ConfigureCryptoHelper(); err != nil {
		return nil, err
	}

	mc.InitChat()

	if err := mc.Greet(); err != nil {
		return nil, err
	}

	if r != nil {
		description := fmt.Sprintf("Matrix Room (%s)", instanceName)
		if err := mc.Register(r, instanceName, description); err != nil {
			return nil, err
		}
	}

	return mc, nil
}
