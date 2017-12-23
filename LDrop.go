package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dustin/go-humanize"
	_ "github.com/kamilkabir9/LDrop/statik" // TODO: Replace with the absolute import path
	"github.com/mdp/qrterminal"
	"github.com/rakyll/statik/fs"
	"github.com/skratchdot/open-golang/open"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	SuccessStatus = "Ok"
	FailedStatus  = "Err"
)

var uploadFolder = "Uploads"

type onlySuffixFilter struct {
	set    bool
	suffix []string
}

var oSF onlySuffixFilter

func (filter *onlySuffixFilter) Set(value string) error {
	filter.set = true
	filter.suffix = strings.Split(value, ",")
	return nil
}
func (filter *onlySuffixFilter) filterFile(fileName string) bool {
	if filter.set {
		for _, v := range filter.suffix {
			if strings.HasSuffix(fileName, v) {
				return false
			}
		}
		fmt.Println("ignore")
		return true
	}
	return false
}

func (filter *onlySuffixFilter) String() string {
	result := "\n"
	for i, v := range filter.suffix {
		result += fmt.Sprintf("%v:%v\n", i, v)
	}
	return result
}

//------------------------------------
type ignorePrefixFilter struct {
	set     bool
	preffix []string
}

var iPF ignorePrefixFilter

func (filter *ignorePrefixFilter) Set(value string) error {
	filter.set = true

	filter.preffix = strings.Split(value, ",")
	return nil
}
func (filter *ignorePrefixFilter) filterFile(fileName string) bool {
	if filter.set {
		for _, v := range filter.preffix {
			if strings.HasPrefix(fileName, v) {
				return true
			}
		}
		return false
	}
	return false
}

func (filter *ignorePrefixFilter) String() string {
	result := "\n"
	for i, v := range filter.preffix {
		result += fmt.Sprintf("%v:%v\n", i, v)
	}
	return result
}

//--------------------------
type ignoreSuffixFilter struct {
	set    bool
	suffix []string
}

var iSF ignoreSuffixFilter

func (filter *ignoreSuffixFilter) Set(value string) error {
	filter.set = true
	filter.suffix = strings.Split(value, ",")
	return nil
}

func (filter *ignoreSuffixFilter) filterFile(fileName string) bool {
	if filter.set {
		for _, v := range filter.suffix {
			if strings.HasSuffix(fileName, v) {
				return true
			}
		}
		return false
	}
	return false
}

func (filter *ignoreSuffixFilter) String() string {
	result := "\n"
	for i, v := range filter.suffix {
		result += fmt.Sprintf("%v:%v\n", i, v)
	}
	return result
}

func filterFile(fileName string) bool {
	if iSF.filterFile(fileName) || iPF.filterFile(fileName) ||(ignoreHiddenFiles&&strings.HasPrefix(fileName,"."))|| oSF.filterFile(fileName) {
		return true
	}
	return false
}

var statikFS http.FileSystem
var ignoreHiddenFolders bool
var ignoreHiddenFiles bool

func main() {
	log.SetFlags(log.Lshortfile)
	wd, err := os.Getwd()
	flag.StringVar(&uploadFolder, "folder", uploadFolder, "Root Folder")
	flag.Var(&iSF, "ignoreSuffix", "input file SUFFIX to exclude")
	flag.Var(&iPF, "ignorePreffix", "input file PREFFIX to exclude")
	flag.Var(&oSF, "onlySuffix", "input file SUFFIX to only to include")
	flag.BoolVar(&ignoreHiddenFolders, "ignoreHiddenFolders", false, "ignoreHiddenFolders")
	flag.BoolVar(&ignoreHiddenFiles, "ignoreHiddenFiles", false, "ignoreHiddenFiles")
	flag.Parse()
	uploadFolder, err := filepath.Abs(uploadFolder)
	if err != nil {
		log.Panicln(err)
	}
	if uploadFolder=="Uploads"{
		if _, err := os.Stat("Uploads"); os.IsNotExist(err) {
			err=os.Mkdir("Uploads",0777)
			if err!=nil{
				log.Panicln(err)
			}
	}
	}
	statikFS, err = fs.New()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/viewFile/", viewFileHandler)
	http.Handle("/", http.FileServer(statikFS))
	http.HandleFunc("/upload", upLoadHandler)
	http.HandleFunc("/getLastFile", getLastFileHandler)
	http.HandleFunc("/getAllFiles", getAllFilesHandler)
	http.HandleFunc("/getFile/", getFileHandler)
	http.HandleFunc("/downLoadFile/", serveThisFileHandler)

	//Adapted from https://stackoverflow.com/questions/43424787/how-to-use-next-available-port-in-http-listenandserve
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	fmt.Printf("Strarting Server...\nworkingDir:%v\nrootFolder:%v\nIP address: %v:%v\nFiltering rules\nremoving Files with Suffix:%v,\nremoving Files with Preffix:%v,\nadding Files ony with Suffix:%v\nHide Hidden Files:%v\nHide Hidden Folders:%v\n", wd, uploadFolder, GetOutboundIP(), port, iSF.String(), iPF.String(), oSF.String(),ignoreHiddenFiles,ignoreHiddenFolders)
	//err = http.ListenAndServe(":"+port, nil)
	qrterminal.Generate(fmt.Sprintf("%v:%v", GetOutboundIP(), port), qrterminal.M, os.Stdout)
	open.Run(fmt.Sprintf("http://%v:%v", GetOutboundIP(), port))
	err = http.Serve(listener, nil)
	if err != nil {
		log.Println("ERR : ", err)
	}
}

//Adapted from https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	//log.Println("getting : " + r.URL.Path[1:])
	http.ServeFile(w, r, r.URL.Path[1:])
}

func UploadStatusJson(status string, desc string) string {
	type resultAsjson struct {
		Status      string
		Description string
	}
	var resultJson = resultAsjson{status, desc}
	result, err := json.Marshal(resultJson)
	if err != nil {
		log.Println("ERR : ", err)
		return fmt.Sprintf("{\"Status\":%v,\"Description\":%v}", FailedStatus, err)
	}
	return string(result)
}

func upLoadHandler(w http.ResponseWriter, r *http.Request) {

	log.Println("upLoadHandler called")
	fmt.Println("Downloading File.....")

	file, fileHeader, err := r.FormFile("fileUpload")
	if err != nil {
		log.Println(err)
		result := UploadStatusJson(FailedStatus, fmt.Sprint(err))
		fmt.Fprint(w, result)
		return
	}

	if _, err := os.Stat(uploadFolder); os.IsNotExist(err) {
		os.Mkdir(uploadFolder, 0777)
	}

	if err != nil {
		log.Println(err)
		result := UploadStatusJson(FailedStatus, fmt.Sprint(err))
		fmt.Fprint(w, result)
		return
	}
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		log.Println(err)
		result := UploadStatusJson(FailedStatus, fmt.Sprint(err))
		fmt.Fprint(w, result)
		return
	}

	uniqfileName := getUniqFileName(fileHeader.Filename)
	of, err := os.Create(uniqfileName)
	if err != nil {
		log.Println("ERR : ", err)
		result := UploadStatusJson(FailedStatus, fmt.Sprint(err))
		fmt.Fprint(w, result)
		return
	}
	of.Write(fileBytes)

	fmt.Printf("File: %v saved at location: %v\n", fileHeader.Filename, uniqfileName)
	result := UploadStatusJson(SuccessStatus, fmt.Sprintf("Uploaded file %v", fileHeader.Filename))
	fmt.Fprint(w, result)
	fmt.Println("Downloaded File : " + fileHeader.Filename)
}

//func getUniqFileName check if file with same file name exists .if yes then creates a new file name
func getUniqFileName(filename string) string {
	uploadFileName := filename
	uploadFileName = filepath.Join(uploadFolder, uploadFileName)
	exists := true
	count := 0
	for exists {
		count += 1
		if _, err := os.Stat(uploadFileName); os.IsNotExist(err) {
			exists = false
		} else {
			//file.png -> file-1.png
			uploadFileName = strings.Replace(uploadFileName, path.Ext(uploadFileName), "-"+strconv.Itoa(count)+path.Ext(uploadFileName), 1)
			log.Println("made uniq!!!!!!!!!!!!")
		}
	}
	return uploadFileName
}

func getLastFileHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("getting Last file..")
	fileList := getAllFiles()
	lastFile := fileList[0]
	for _, file := range fileList {
		if lastFile.Info.ModTime.Before(file.Info.ModTime) {
			lastFile = file
		}
	}
	fmt.Println("Last file:", lastFile.Name)
	//Adapted from https://stackoverflow.com/questions/31638447/how-to-server-a-file-from-a-handler-in-golang
	w.Header().Set("Content-Description", "File Transfer")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Content-Disposition", "attachment; filename="+lastFile.Info.Name)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path.Join(uploadFolder, lastFile.Name))
}

type osFileInfo struct {
	Name    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
}
type fileInfo struct {
	Name    string
	ModTime string
	Size    string
	Info    osFileInfo
}

var wg sync.WaitGroup
var mx sync.Mutex

func getAllFiles() []fileInfo {
	var fileNamesWithTime = new([]fileInfo)
	wg.Add(1)
	getAllFilesConcurrent(uploadFolder, fileNamesWithTime)
	wg.Wait()
	fmt.Println("completed reading Folder")
	log.Println("Total nuber of Files: ", len(*fileNamesWithTime))
	return *fileNamesWithTime
}
func getAllFilesConcurrent(Dir string, fileNamesWithTime *[]fileInfo) {
	fmt.Println("Reading Dir: ", Dir)
	fileList, err := ioutil.ReadDir(Dir)
	if err != nil {
		log.Panicln("ERR : ", err)
	}
	for _, file := range fileList {

		if !file.IsDir() {
			var fileNameKey string
			if filterFile(file.Name()) {
				continue
			}
			fileNameKey = filepath.Join(Dir, file.Name())
			fileNameKey = strings.Replace(fileNameKey, uploadFolder+string(os.PathSeparator), "", 1)
			mx.Lock()
			*fileNamesWithTime = append(*fileNamesWithTime, fileInfo{fileNameKey, file.ModTime().Format(time.ANSIC), humanize.Bytes(uint64(file.Size())), osFileInfo{file.Name(), file.Size(), file.Mode(), file.ModTime(), file.IsDir()}})
			mx.Unlock()
		} else {
			if !(ignoreHiddenFolders && strings.HasPrefix(file.Name(), ".")) {
				wg.Add(1)
				go getAllFilesConcurrent(filepath.Join(Dir, file.Name()), fileNamesWithTime)
			}else if !ignoreHiddenFolders{
				wg.Add(1)
				go getAllFilesConcurrent(filepath.Join(Dir, file.Name()), fileNamesWithTime)
			}
		}
	}
	wg.Done()
}

func getAllFilesHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("getting All files..")
	//var fileNamesWithTime = getAllFiles(uploadFolder)
	var fileNamesWithTime = getAllFiles()
	fileNamesJson, err := json.Marshal(fileNamesWithTime)
	if err != nil {
		log.Println("ERR : ", err)
		result := UploadStatusJson(FailedStatus, fmt.Sprint(err))
		fmt.Fprintln(w, result)
		return
	}
	result := UploadStatusJson(SuccessStatus, string(fileNamesJson))
	fmt.Fprintln(w, result)
	return
}

func getFileHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("getFileHandler")
	fileName := r.URL.Path
	fileName = strings.Replace(fileName, "/getFile/", "", -1)
	fileName, err := url.QueryUnescape(fileName)
	if err != nil {
		log.Println(err)
	}
	log.Println(fileName)
	fmt.Println("getting File : ", fileName)
	http.ServeFile(w, r, path.Join(uploadFolder, fileName))
	//http.ServeContent(w, r, path.Join(uploadFolder, fileName))

}
func serveThisFileHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("getFileHandler")
	fileName := r.URL.Path
	fileName = strings.Replace(fileName, "/downLoadFile/", "", -1)
	fileName, err := url.QueryUnescape(fileName)
	if err != nil {
		log.Println(err)
	}
	//TODO files with sapce not working
	log.Println(fileName)
	fmt.Println("getting File : ", fileName)
	//Adapted from https://stackoverflow.com/questions/31638447/how-to-server-a-file-from-a-handler-in-golang
	w.Header().Set("Content-Description", "File Transfer")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path.Join(uploadFolder, fileName))
}
func viewFileHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("viewFileHandler")
	f, err := statikFS.Open("/viewFile.html")
	if err != nil {
		log.Println(err)
	}
	http.ServeContent(w, r, "viewFile.html", time.Now(), f)
}