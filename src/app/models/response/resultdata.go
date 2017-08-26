package response

//返回前端的数据结构
type ResultData struct {
	IsSuccess bool
	Msg       string
	Data      interface{}
}

//开始，结束返回给前端的实体
type JobInfo struct {
	Status int
	Prev   string
	Next   string
}

//上传文件的相关信息
type UploadFileInfo struct {
	OldFileName string
	NewFileName string
}