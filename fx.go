package fxdb

import (
	"context"
	config "github.com/templatedop/fxconfig"
	logger "github.com/templatedop/fxlogger"
	"go.uber.org/fx"
)

var FxDBModule = fx.Module("database",
	fx.Provide(NewDBConfig,fx.Private) ,
	fx.Provide(
		//NewDBConfig ,
		Pgxconfig,
		NewDB,
	),

	fx.Invoke(func(db *DB, log *logger.Logger, c config.Econfig, lc fx.Lifecycle) error {

		log.Debug("Inside fx invoke DB")

		lc.Append(fx.Hook{

			OnStart: func(ctx context.Context) error {
				log.Debug("Inside fxdb/fx.go")
				err := db.Ping(ctx)
				if err != nil {
					return err
				}

				log.Info("Successfully connected to the database")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				db.Close()
				return nil
			},
		})
		return nil
	}),
)
