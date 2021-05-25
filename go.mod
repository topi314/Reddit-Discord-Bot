module github.com/TopiSenpai/Reddit-Discord-Bot

go 1.15

replace github.com/DisgoOrg/disgo => ../disgo

require (
	github.com/DisgoOrg/disgo v0.3.2
	github.com/DisgoOrg/disgohook v1.0.4
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/lib/pq v1.10.2 // indirect
	github.com/prometheus/common v0.25.0 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/vartanbeno/go-reddit/v2 v2.0.0
	golang.org/x/sys v0.0.0-20210521203332-0cec03c779c1 // indirect
	gorm.io/driver/postgres v1.1.0
	gorm.io/gorm v1.21.10
	gorm.io/plugin/prometheus v0.0.0-20210507023802-dc84a49b85d1 // indirect
)
