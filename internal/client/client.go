package client

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/jkimbo/stacked/internal/config"
	"github.com/jkimbo/stacked/internal/db"
)

type StackedClient struct {
	SQLDB  *db.SQLDB
	Config *config.Config
}

func (c *StackedClient) DefaultBranch() string {
	return c.Config.DefaultBranch
}

func NewStackedClient(ctx context.Context) (*StackedClient, error) {
	sqlDB, err := db.NewDB(ctx, filepath.Join(".stacked", "main.db"))
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to database: %v\n", err)
	}

	config, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	return &StackedClient{
		SQLDB:  sqlDB,
		Config: config,
	}, nil
}
