package pkg

import "go.uber.org/zap"

var Log *zap.Logger

func LoggerInit(){
	var err error
	Log,err = zap.NewProduction()
	if err!=nil{
		panic("Faild to initialize logger")
	}
	defer Log.Sync()
}	


