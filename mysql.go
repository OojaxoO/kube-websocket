package main

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/labstack/gommon/log"
	"github.com/spf13/viper"
	"io/ioutil"
)

type Cluster struct {
	ID string
	Name string
	Config string
	Comment string
}

func (c Cluster) TableName() string {
	return "cluster"
}

func getKubeConfigFile(id int) string {
	db, err := gorm.Open("mysql", viper.GetString("mysql"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	var cluster Cluster
	db.First(&cluster, id)
	dir, err := ioutil.TempFile("/tmp/", "config")
	if err != nil {
		log.Fatal(err)
	}
	_ = ioutil.WriteFile(dir.Name(), []byte(cluster.Config), 0644)
	return dir.Name()
}