package main

import (
	"app/controllers"
	"app/jobs"
	_ "app/mail"
	"app/models"
	"html/template"
	"net/http"

	"github.com/astaxie/beego"
	//"fmt"
	//"github.com/astaxie/beego/logs"
)

const VERSION = "1.0.0"

func main() {

	/* 日志的使用
	log := logs.NewLogger()
	log.SetLogger(logs.AdapterFile, `{"filename":"test.log","daily":true}`)
	log.Async(6)
	fmt.Println("log start:")
	for i :=0; i < 10; i++ {
		log.Trace("test first one: %d",i)
		log.Debug("test debug")
	}
	fmt.Println("log end:")
	*/

	models.Init()
	jobs.InitJobs()

	// 设置默认404页面
	beego.ErrorHandler("404", func(rw http.ResponseWriter, r *http.Request) {
		t, _ := template.New("404.html").ParseFiles(beego.BConfig.WebConfig.ViewsPath + "/error/404.html")
		data := make(map[string]interface{})
		data["content"] = "page not found"
		t.Execute(rw, data)
	})

	// 生产环境不输出debug日志
	if beego.AppConfig.String("runmode") == "prod" {
		beego.SetLevel(beego.LevelInformational)
	}
	beego.AppConfig.Set("version", VERSION)

	// 路由设置
	beego.Router("/", &controllers.MainController{}, "*:Index")
	beego.Router("/login", &controllers.MainController{}, "*:Login")
	beego.Router("/logout", &controllers.MainController{}, "*:Logout")
	beego.Router("/profile", &controllers.MainController{}, "*:Profile")
	beego.Router("/gettime", &controllers.MainController{}, "*:GetTime")
	beego.Router("/help", &controllers.HelpController{}, "*:Index")
	beego.AutoRouter(&controllers.TaskController{})
	beego.AutoRouter(&controllers.GroupController{})

	beego.BConfig.WebConfig.Session.SessionOn = true
	beego.Run()

}
