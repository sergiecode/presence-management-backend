package logger

import (
	"go.uber.org/zap"
)

var Log *zap.Logger

func Init() error {
	var err error
	Log, err = zap.NewProduction() // or zap.NewDevelopment() for dev
	return err
}
