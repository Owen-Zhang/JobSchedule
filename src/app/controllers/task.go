package controllers

import (
	"github.com/astaxie/beego"
	"github.com/robfig/cron"
	"app/jobs"
	"app/libs"
	"app/models"
	"strconv"
	"strings"
	"time"
	"app/models/response"
	"os"
	"github.com/mholt/archiver"
)

type TaskController struct {
	BaseController
}

var upload models.Uploadfile

func init(){
	upload.Tempfilepath = beego.AppConfig.String("upload.tempfolder")
	upload.Runtimepath = beego.AppConfig.String("upload.runtimefolder")
}

// 任务列表
func (this *TaskController) List() {
	page, _ := this.GetInt("page")
	if page < 1 {
		page = 1
	}
	groupId, _ := this.GetInt("groupid")
	filters := make([]interface{}, 0)
	if groupId > 0 {
		filters = append(filters, "group_id", groupId)
	}
	result, count := models.TaskGetList(page, this.pageSize, filters...)

	list := make([]map[string]interface{}, len(result))
	for k, v := range result {
		row := make(map[string]interface{})
		row["id"] = v.Id
		row["name"] = v.TaskName
		row["cron_spec"] = v.CronSpec
		row["status"] = v.Status
		row["description"] = v.Description

		e := jobs.GetEntryById(v.Id)
		if e != nil {
			row["next_time"] = beego.Date(e.Next, "Y-m-d H:i:s")
			row["prev_time"] = "-"
			if e.Prev.Unix() > 0 {
				row["prev_time"] = beego.Date(e.Prev, "Y-m-d H:i:s")
			} else if v.PrevTime > 0 {
				row["prev_time"] = beego.Date(time.Unix(v.PrevTime, 0), "Y-m-d H:i:s")
			}
			row["running"] = 1
		} else {
			row["next_time"] = "-"
			if v.PrevTime > 0 {
				row["prev_time"] = beego.Date(time.Unix(v.PrevTime, 0), "Y-m-d H:i:s")
			} else {
				row["prev_time"] = "-"
			}
			row["running"] = 0
		}
		list[k] = row
	}

	// 分组列表
	groups, _ := models.TaskGroupGetList(1, 100)

	this.Data["pageTitle"] = "任务列表"
	this.Data["list"] = list
	this.Data["groups"] = groups
	this.Data["groupid"] = groupId
	this.Data["pageBar"] = libs.NewPager(page, int(count), this.pageSize, beego.URLFor("TaskController.List", "groupid", groupId), true).ToString()
	this.display()
}

// 上传要运行的文件
func(this *TaskController) UploadRunFile() {
	f, h, err := this.GetFile("files[]")
	defer f.Close()

	uploadResult := &response.ResultData{
		IsSuccess: false,
	}

	if err != nil {
		uploadResult.Msg = "请选择要上传的文件"
		this.Data["json"] = uploadResult
		this.ServeJSON()
		return

	} else {
		fileTool := &libs.FileTool{Url:h.Filename}
		exts := []string {"zip"} //这个要从配制文件中去取
		if !fileTool.CheckFileExt(exts) {
			uploadResult.Msg = "请上传正确的文件类型"
			this.Data["json"] = uploadResult
			this.ServeJSON()
			return
		}

		uuidFileName := fileTool.CreateUuidFile()
		if uuidFileName == "" {
			uploadResult.Msg = "文件保存出错，请重新选择文件"
			this.Data["json"] = uploadResult
			this.ServeJSON()
			return
		}

		filePath := upload.Tempfilepath + uuidFileName
		os.MkdirAll(upload.Tempfilepath,0777)
		this.SaveToFile("files[]", filePath)

		uploadResult.IsSuccess = true
		uploadResult.Data = &response.UploadFileInfo{
			OldFileName: fileTool.Url,
			NewFileName: filePath,
		}
		this.jsonResult(uploadResult)
	}
}

// 添加任务
func (this *TaskController) Add() {
	// 分组列表
	groups, _ := models.TaskGroupGetList(1, 100)
	this.Data["groups"] = groups
	this.Data["pageTitle"] = "添加任务"
	this.display()
}

//保存任务
func (this *TaskController) SaveTask() {
	task := new(models.Task)
	task.UserId = this.userId
	task.GroupId, _ = this.GetInt("group_id")
	task.TaskName = strings.TrimSpace(this.GetString("task_name"))
	task.Description = strings.TrimSpace(this.GetString("description"))
	task.Concurrent, _ = this.GetInt("concurrent")
	task.CronSpec = strings.TrimSpace(this.GetString("cron_spec"))
	task.Command = strings.TrimSpace(this.GetString("command"))
	task.Notify, _ = this.GetInt("notify")
	task.Timeout, _ = this.GetInt("timeout")
	runFileName := strings.TrimSpace(this.GetString("runfilename"))
	resultData := &response.ResultData{IsSuccess:false, Msg:"",}
	notifyEmail := strings.TrimSpace(this.GetString("notify_email"))
	if notifyEmail != "" {
		emailList := make([]string, 0)
		tmp := strings.Split(notifyEmail, ";")
		for _, v := range tmp {
			v = strings.TrimSpace(v)
			if !libs.IsEmail([]byte(v)) {
				resultData.Msg = "无效的Email地址："+v
				this.jsonResult(resultData)
			} else {
				emailList = append(emailList, v)
			}
		}
		task.NotifyEmail = strings.Join(emailList, ";")
	}

	if task.TaskName == "" || task.CronSpec == "" || task.Command == "" {
		resultData.Msg = "请填写完整信息"
		this.jsonResult(resultData)
	}
	if _, err := cron.Parse(task.CronSpec); err != nil {
		resultData.Msg = "cron表达式无效"
		this.jsonResult(resultData)
	}

	//解压文件
	runfileFolder, err2 := this.unzipUploadFile(runFileName)
	if err2 != nil {
		resultData.Msg = err2.Error()
		this.jsonResult(resultData)
	}

	//保存数据库
	task.RunFileName = runfileFolder
	if _, err := models.TaskAdd(task); err != nil {
		resultData.Msg = err.Error()
		this.jsonResult(resultData)
	}

	resultData.IsSuccess = true
	this.jsonResult(resultData)
}

func (this *TaskController) unzipUploadFile(filePath string) (string, error)  {
	if filePath == "" {
		return "", nil
	}

	fileTool := &libs.FileTool{Url:filePath}
	if (fileTool.IsExist()) {
		runFileFolder := upload.Runtimepath + fileTool.FileName() + "/"
		if err := os.MkdirAll(runFileFolder,0777); err != nil {
			return "", err
		}
		if err2 := archiver.Zip.Open(filePath, runFileFolder); err2 != nil {
			return "", err2
		}
		return runFileFolder, nil
	}
	return "", nil
}

// 编辑任务
func (this *TaskController) Edit() {
	id, _ := this.GetInt("id")

	task, err := models.TaskGetById(id)
	if err != nil {
		this.showMsg(err.Error())
	}

	if this.isPost() {
		task.TaskName = strings.TrimSpace(this.GetString("task_name"))
		task.Description = strings.TrimSpace(this.GetString("description"))
		task.GroupId, _ = this.GetInt("group_id")
		task.Concurrent, _ = this.GetInt("concurrent")
		task.CronSpec = strings.TrimSpace(this.GetString("cron_spec"))
		task.Command = strings.TrimSpace(this.GetString("command"))
		task.Notify, _ = this.GetInt("notify")
		task.Timeout, _ = this.GetInt("timeout")

		notifyEmail := strings.TrimSpace(this.GetString("notify_email"))
		if notifyEmail != "" {
			tmp := strings.Split(notifyEmail, "\n")
			emailList := make([]string, 0, len(tmp))
			for _, v := range tmp {
				v = strings.TrimSpace(v)
				if !libs.IsEmail([]byte(v)) {
					this.ajaxMsg("无效的Email地址："+v, MSG_ERR)
				} else {
					emailList = append(emailList, v)
				}
			}
			task.NotifyEmail = strings.Join(emailList, "\n")
		}

		if task.TaskName == "" || task.CronSpec == "" || task.Command == "" {
			this.ajaxMsg("请填写完整信息", MSG_ERR)
		}
		if _, err := cron.Parse(task.CronSpec); err != nil {
			this.ajaxMsg("cron表达式无效", MSG_ERR)
		}
		if err := task.Update(); err != nil {
			this.ajaxMsg(err.Error(), MSG_ERR)
		}

		this.ajaxMsg("", MSG_OK)
	}

	// 分组列表
	groups, _ := models.TaskGroupGetList(1, 100)
	this.Data["groups"] = groups
	this.Data["task"] = task
	this.Data["pageTitle"] = "编辑任务"
	this.display()
}

// 任务执行日志列表
func (this *TaskController) Logs() {
	taskId, _ := this.GetInt("id")
	page, _ := this.GetInt("page")
	if page < 1 {
		page = 1
	}

	task, err := models.TaskGetById(taskId)
	if err != nil {
		this.showMsg(err.Error())
	}

	result, count := models.TaskLogGetList(page, this.pageSize, "task_id", task.Id)

	list := make([]map[string]interface{}, len(result))
	for k, v := range result {
		row := make(map[string]interface{})
		row["id"] = v.Id
		row["start_time"] = beego.Date(time.Unix(v.CreateTime, 0), "Y-m-d H:i:s")
		row["process_time"] = float64(v.ProcessTime) / 1000
		row["ouput_size"] = libs.SizeFormat(float64(len(v.Output)))
		row["status"] = v.Status
		list[k] = row
	}

	this.Data["pageTitle"] = "任务执行日志"
	this.Data["list"] = list
	this.Data["task"] = task
	this.Data["pageBar"] = libs.NewPager(page, int(count), this.pageSize, beego.URLFor("TaskController.Logs", "id", taskId), true).ToString()
	this.display()
}

// 查看日志详情
func (this *TaskController) ViewLog() {
	id, _ := this.GetInt("id")

	taskLog, err := models.TaskLogGetById(id)
	if err != nil {
		this.showMsg(err.Error())
	}

	task, err := models.TaskGetById(taskLog.TaskId)
	if err != nil {
		this.showMsg(err.Error())
	}

	data := make(map[string]interface{})
	data["id"] = taskLog.Id
	data["output"] = taskLog.Output
	data["error"] = taskLog.Error
	data["start_time"] = beego.Date(time.Unix(taskLog.CreateTime, 0), "Y-m-d H:i:s")
	data["process_time"] = float64(taskLog.ProcessTime) / 1000
	data["ouput_size"] = libs.SizeFormat(float64(len(taskLog.Output)))
	data["status"] = taskLog.Status

	this.Data["task"] = task
	this.Data["data"] = data
	this.Data["pageTitle"] = "查看日志"
	this.display()
}

// 批量操作日志
func (this *TaskController) LogBatch() {
	action := this.GetString("action")
	ids := this.GetStrings("ids")
	if len(ids) < 1 {
		this.ajaxMsg("请选择要操作的项目", MSG_ERR)
	}
	for _, v := range ids {
		id, _ := strconv.Atoi(v)
		if id < 1 {
			continue
		}
		switch action {
		case "delete":
			models.TaskLogDelById(id)
		}
	}

	this.ajaxMsg("", MSG_OK)
}

// 批量操作
func (this *TaskController) Batch() {
	action := this.GetString("action")
	ids := this.GetStrings("ids")
	if len(ids) < 1 {
		this.ajaxMsg("请选择要操作的项目", MSG_ERR)
	}

	for _, v := range ids {
		id, _ := strconv.Atoi(v)
		if id < 1 {
			continue
		}
		switch action {
		case "active":
			if task, err := models.TaskGetById(id); err == nil {
				job, err := jobs.NewJobFromTask(task)
				if err == nil {
					jobs.AddJob(task.CronSpec, job)
					task.Status = 1
					task.Update()
				}
			}
		case "pause":
			jobs.RemoveJob(id)
			if task, err := models.TaskGetById(id); err == nil {
				task.Status = 0
				task.Update()
			}
		case "delete":
			models.TaskDel(id)
			models.TaskLogDelByTaskId(id)
			jobs.RemoveJob(id)
		}
	}

	this.ajaxMsg("", MSG_OK)
}

// 启动任务
func (this *TaskController) Start() {
	id, _ := this.GetInt("id")

	task, err := models.TaskGetById(id)
	if err != nil {
		this.showMsg(err.Error())
	}

	job, err := jobs.NewJobFromTask(task)
	if err != nil {
		this.showMsg(err.Error())
	}

	if jobs.AddJob(task.CronSpec, job) {
		task.Status = 1
		task.Update()
	}

	startJob := jobs.GetEntryById(id)
	this.Data["json"] = &response.ResultData{
		IsSuccess: true,
		Msg: "",
		Data: &response.JobInfo{
			Status:1,
			Prev:time.Unix(task.PrevTime, 0).Format("2006-01-02 15:04:05"),
			Next:beego.Date(startJob.Next, "Y-m-d H:i:s"),
		},
	}
	this.ServeJSON()
}

// 暂停任务
func (this *TaskController) Pause() {
	id, _ := this.GetInt("id")

	task, err := models.TaskGetById(id)
	if err != nil {
		this.showMsg(err.Error())
	}

	jobs.RemoveJob(id)
	task.Status = 0
	task.Update()

	this.Data["json"] = &response.ResultData{
		IsSuccess: true,
		Msg: "",
		Data: &response.JobInfo{
			Status:0,
			Prev: time.Unix(task.PrevTime, 0).Format("2006-01-02 15:04:05"),
			Next:"-",
		},
	}
	this.ServeJSON()
}

// 立即执行
func (this *TaskController) Run() {
	id, _ := this.GetInt("id")

	entry := jobs.GetEntryById(id)
	if entry == nil {
		this.showMsg("没有找到相关的任务。")
	}
	entry.Job.Run()

	startJob := jobs.GetEntryById(id)
	task, _ := models.TaskGetById(id)

	this.Data["json"] = &response.ResultData{
		IsSuccess: true,
		Msg: "",
		Data: &response.JobInfo{
			Status:1,
			Prev:time.Unix(task.PrevTime, 0).Format("2006-01-02 15:04:05"),
			Next:beego.Date(startJob.Next, "Y-m-d H:i:s"),
		},
	}
	this.ServeJSON()
}

// 删除任务，同时删除数据库中的Task
func (this *TaskController) Delete() {
	id, _ := this.GetInt("id")
	jobs.RemoveJob(id)
	models.TaskDel(id)

	this.Data["json"] = &response.ResultData{
		IsSuccess: true,
		Msg: "",
		Data: true,
	}
	this.ServeJSON()
}
