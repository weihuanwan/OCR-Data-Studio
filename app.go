package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/weihuanwan/paddleocr-go/common"
	"github.com/weihuanwan/paddleocr-go/layout"
	"github.com/weihuanwan/paddleocr-go/ocr"
	"github.com/weihuanwan/paddleocr-go/utils"
	"github.com/weihuanwan/paddleocr-go/vl"
	ort "github.com/yalue/onnxruntime_go"
	"gocv.io/x/gocv"
)

// ================= 1. 配置结构体 =================
type AppConfig struct {
	ConfigPath string `json:"-"`
	Url        string `json:"url"`
	ModelName  string `json:"modelName"`
	ApiKey     string `json:"apiKey"`
}

// ================= 2. 最近使用文件结构体 =================
type RecentFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Date     string `json:"date"`
	Size     string `json:"size"`
	Favorite bool   `json:"favorite"`
	LastUsed int64  `json:"lastUsed"`
}

// ================= 3. App =================
type App struct {
	ctx         context.Context
	paddleOCRVL *vl.PaddleOCRVL
	config      *AppConfig
	tempDir     string
	recentPath  string
	cacheDir    string
	sessionMu   sync.Mutex
	// 👇 新增：日志相关字段
	logger  *slog.Logger
	logFile *os.File
}

func NewApp(tempDir string) *App {
	app := &App{
		tempDir: tempDir,
	}

	// 👇 新增：在初始化配置前，先初始化日志系统
	app.initLogger()

	app.initConfig()
	return app
}

func (a *App) initLogger() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}

	logDir := filepath.Join(configDir, "OCR-Data-Studio", "logs")
	_ = os.MkdirAll(logDir, 0755)

	// 按天命名日志文件，例如 app_2026-09-04.log
	logFileName := fmt.Sprintf("app_%s.log", time.Now().Format("2006-01-02"))
	logFilePath := filepath.Join(logDir, logFileName)

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		a.logger.Info("⚠️ 无法创建日志文件 %s: %v\n", logFilePath, err)
		// 降级：如果文件创建失败，只输出到控制台
		a.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
		return
	}

	a.logFile = file

	// 🌟 核心：使用 MultiWriter 同时输出到 控制台(os.Stdout) 和 文件(file)
	multiWriter := io.MultiWriter(os.Stdout, file)

	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug, // 记录 Debug 及以上级别 (Debug, Info, Warn, Error)
	})

	a.logger = slog.New(handler)
	a.logger.Info("🚀 日志系统初始化成功", "logPath", logFilePath)
}

// ================= 4. 本地文件服务 =================
// /local/ 临时目录
// /cache/ 持久化缓存目录
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path

	var base string
	var reqPath string

	switch {
	case strings.HasPrefix(urlPath, "/local/"):
		base = a.tempDir
		reqPath = strings.TrimPrefix(urlPath, "/local/")
	case strings.HasPrefix(urlPath, "/cache/"):
		base = a.cacheDir
		reqPath = strings.TrimPrefix(urlPath, "/cache/")
	default:
		http.NotFound(w, r)
		return
	}

	cleaned := filepath.Clean("/" + reqPath)
	fullPath := filepath.Join(base, cleaned)

	if !strings.HasPrefix(fullPath, base) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, fullPath)
}

// ================= 5. 初始化配置 =================
func (a *App) initConfig() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}

	appConfigDir := filepath.Join(configDir, "OCR-Data-Studio")
	_ = os.MkdirAll(appConfigDir, 0755)

	configPath := filepath.Join(appConfigDir, "config.json")

	a.recentPath = filepath.Join(appConfigDir, "recent.json")
	a.cacheDir = filepath.Join(appConfigDir, "cache")

	_ = os.MkdirAll(a.cacheDir, 0755)

	a.config = &AppConfig{
		ConfigPath: configPath,
		ModelName:  "AuditAid/PaddleOCR-VL-1.6-0.9B",
		Url:        "http://localhost:11434/v1/chat/completions",
		ApiKey:     "sk-ufajxhcyibsxcatybmjqhaierwwbbxjdrhwitcmrscyodhsq",
	}

	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, a.config)
	} else {
		_ = a.saveConfig()
	}

	a.config.ConfigPath = configPath
}

func (a *App) saveConfig() error {
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.config.ConfigPath, data, 0644)
}

//go:embed all:lib
var embeddedLibFS embed.FS

// extractLibFiles 将打包进二进制的 lib 文件释放到本地磁盘，并返回真实的绝对路径
func (a *App) extractLibFiles() (libPath string, layoutPath string, err error) {
	// 获取用户的配置目录 (Windows: %APPDATA%, Mac: ~/Library/Application Support)
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}

	// 创建一个专属的缓存目录，避免和其他软件冲突
	targetDir := filepath.Join(configDir, "OCR-Data-Studio", "runtime_libs")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", "", fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 1. 释放 onnxruntime.dll
	dllName := "onnxruntime.dll"
	dllDst := filepath.Join(targetDir, dllName)
	// 如果文件不存在，或者大小为0，则从内存释放到磁盘
	if info, err := os.Stat(dllDst); err != nil || info.Size() == 0 {
		data, err := embeddedLibFS.ReadFile("lib/" + dllName)
		if err != nil {
			return "", "", fmt.Errorf("读取内置 %s 失败: %w", dllName, err)
		}
		if err := os.WriteFile(dllDst, data, 0755); err != nil {
			return "", "", fmt.Errorf("写入 %s 失败: %w", dllDst, err)
		}
		a.logger.Info("✅ 已释放内置文件: %s\n", dllDst)
	}

	// 2. 释放 PP-DocLayoutV3.onnx
	onnxName := "PP-DocLayoutV3.onnx"
	onnxDst := filepath.Join(targetDir, onnxName)
	if info, err := os.Stat(onnxDst); err != nil || info.Size() == 0 {
		data, err := embeddedLibFS.ReadFile("lib/" + onnxName)
		if err != nil {
			return "", "", fmt.Errorf("读取内置 %s 失败: %w", onnxName, err)
		}
		if err := os.WriteFile(onnxDst, data, 0755); err != nil {
			return "", "", fmt.Errorf("写入 %s 失败: %w", onnxDst, err)
		}
		a.logger.Info("✅ 已释放内置文件: %s\n", onnxDst)
	}

	return dllDst, onnxDst, nil
}

// ================= 6. OCR Session =================
func (a *App) ensureSession() error {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()

	if a.paddleOCRVL != nil {
		return nil
	}

	if a.config == nil {
		return fmt.Errorf("配置未初始化")
	}

	// 1. 将打包的文件释放到真实磁盘，获取绝对路径
	libPath, layoutPath, err := a.extractLibFiles()
	if err != nil {
		// 如果释放失败（比如在 wails dev 开发环境下没有 embed 进去），回退到本地相对路径
		a.logger.Info("⚠️ 释放内置文件失败，回退到本地相对路径: %v\n", err)
		libPath = "./lib/onnxruntime.dll"
		layoutPath = "./lib/PP-DocLayoutV3.onnx"
	}

	// 2. 直接调用 ocr.InitOrt，它内部会自动处理 InitializeEnvironment，千万不要在外面再调一次！
	if err := ocr.InitOrt(libPath); err != nil {
		return fmt.Errorf("初始化 ORT 失败: %w", err)
	}

	options, err := ort.NewSessionOptions()
	if err != nil {
		return fmt.Errorf("创建 SessionOptions 失败: %w", err)
	}
	defer options.Destroy()

	layoutDetSessionInternal, err := ort.NewDynamicAdvancedSession(
		layoutPath, // 使用真实的磁盘绝对路径
		[]string{"im_shape", "image", "scale_factor"},
		[]string{"fetch_name_0", "fetch_name_1", "fetch_name_2"},
		options,
	)
	if err != nil {
		return fmt.Errorf("创建 Layout Session 失败: %w", err)
	}

	docLayoutSession := layout.NewLayoutDetSession(layoutDetSessionInternal)

	a.paddleOCRVL = vl.NewDefaultPaddleOCRVL(
		a.config.ModelName,
		a.config.Url,
		a.config.ApiKey,
		docLayoutSession,
	)

	return nil
}

// ================= 7. 配置相关接口 =================
func (a *App) GetConfig() *AppConfig {
	return a.config
}

func (a *App) UpdateConfig(newConfig *AppConfig) error {
	if newConfig == nil {
		return fmt.Errorf("配置为空")
	}

	newConfig.ConfigPath = a.config.ConfigPath

	a.config = newConfig

	return a.saveConfig()
}

func (a *App) SelectDirectory(title string) (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
}

func (a *App) SelectFile(title string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "库与模型文件 (*.dll;*.so;*.dylib;*.pdmodel;*.onnx;*.bin)",
				Pattern:     "*.dll;*.so;*.dylib;*.pdmodel;*.onnx;*.bin",
			},
			{
				DisplayName: "所有文件 (*.*)",
				Pattern:     "*.*",
			},
		},
	})
}

func (a *App) SelectExportDir() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择导出目录",
	})
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ================= 8. 最近使用/收藏 =================
func fileIDFromPath(path string) string {
	sum := sha1.Sum([]byte(path))
	return hex.EncodeToString(sum[:])
}

func formatFileSize(size int64) string {
	mb := float64(size) / 1024 / 1024
	if mb >= 1 {
		return fmt.Sprintf("%.2f MB", mb)
	}
	kb := float64(size) / 1024
	return fmt.Sprintf("%.2f KB", kb)
}

func fileTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return "image"
	case ".pdf":
		return "pdf"
	default:
		return "file"
	}
}

func normalizeRecentFile(f RecentFile) RecentFile {
	f.Path = strings.TrimSpace(f.Path)
	if f.Path == "" {
		return f
	}

	if f.ID == "" {
		f.ID = fileIDFromPath(f.Path)
	}

	if info, err := os.Stat(f.Path); err == nil {
		if f.Name == "" {
			f.Name = info.Name()
		}
		if f.Date == "" {
			f.Date = info.ModTime().Format("2006-01-02 15:04")
		}
		if f.Size == "" {
			f.Size = formatFileSize(info.Size())
		}
	} else {
		if f.Name == "" {
			f.Name = filepath.Base(f.Path)
		}
		if f.Date == "" {
			f.Date = time.Now().Format("2006-01-02 15:04")
		}
		if f.Size == "" {
			f.Size = "0 KB"
		}
	}

	if f.Type == "" {
		f.Type = fileTypeFromPath(f.Path)
	}

	if f.LastUsed == 0 {
		f.LastUsed = time.Now().UnixMilli()
	}

	return f
}

func (a *App) loadRecentFiles() []RecentFile {
	if a.recentPath == "" {
		return []RecentFile{}
	}

	data, err := os.ReadFile(a.recentPath)
	if err != nil {
		return []RecentFile{}
	}

	var files []RecentFile
	if err := json.Unmarshal(data, &files); err != nil {
		return []RecentFile{}
	}

	return files
}

func (a *App) saveRecentFiles(files []RecentFile) error {
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.recentPath, data, 0644)
}

func (a *App) GetRecentFiles() ([]RecentFile, error) {
	files := a.loadRecentFiles()

	sort.SliceStable(files, func(i, j int) bool {
		return files[i].LastUsed > files[j].LastUsed
	})

	return files, nil
}

func (a *App) AddRecentFiles(files []RecentFile) ([]RecentFile, error) {
	if len(files) == 0 {
		return a.GetRecentFiles()
	}

	existing := a.loadRecentFiles()
	existingByPath := make(map[string]RecentFile)
	for _, f := range existing {
		existingByPath[f.Path] = f
	}

	sort.SliceStable(existing, func(i, j int) bool {
		return existing[i].LastUsed > existing[j].LastUsed
	})

	var result []RecentFile
	seen := make(map[string]bool)
	now := time.Now().UnixMilli()

	for i, f := range files {
		f = normalizeRecentFile(f)
		if f.Path == "" || seen[f.Path] {
			continue
		}

		if old, ok := existingByPath[f.Path]; ok {
			f.Favorite = old.Favorite
			if f.ID == "" {
				f.ID = old.ID
			}
		}

		f.LastUsed = now + int64(i)
		result = append(result, f)
		seen[f.Path] = true
	}

	for _, f := range existing {
		if seen[f.Path] {
			continue
		}
		result = append(result, f)
		seen[f.Path] = true
	}

	if len(result) > 100 {
		result = result[:100]
	}

	if err := a.saveRecentFiles(result); err != nil {
		return nil, err
	}

	return result, nil
}

func (a *App) ToggleFavorite(id string) ([]RecentFile, error) {
	files := a.loadRecentFiles()

	for i := range files {
		if files[i].ID == id || files[i].Path == id {
			files[i].Favorite = !files[i].Favorite
		}
	}

	sort.SliceStable(files, func(i, j int) bool {
		return files[i].LastUsed > files[j].LastUsed
	})

	if err := a.saveRecentFiles(files); err != nil {
		return nil, err
	}

	return files, nil
}

func (a *App) RemoveRecentFile(id string) ([]RecentFile, error) {
	files := a.loadRecentFiles()
	var result []RecentFile

	for _, f := range files {
		if f.ID == id || f.Path == id {
			_ = a.DeleteParseResult(f.Path)
			continue
		}
		result = append(result, f)
	}

	if err := a.saveRecentFiles(result); err != nil {
		return nil, err
	}

	return result, nil
}

func (a *App) ClearRecentFiles() ([]RecentFile, error) {
	err := a.saveRecentFiles([]RecentFile{})
	if err != nil {
		return nil, err
	}
	return []RecentFile{}, nil
}

// ================= 9. 打开文件 =================
func (a *App) OpenFiles() ([]RecentFile, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("app context is nil")
	}

	filePaths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "所有支持格式 (*.pdf;*.jpg;*.jpeg;*.png)",
				Pattern:     "*.pdf;*.jpg;*.jpeg;*.png",
			},
			{
				DisplayName: "所有文件 (*.*)",
				Pattern:     "*.*",
			},
		},
	})

	if err != nil {
		return nil, err
	}

	if len(filePaths) == 0 {
		return nil, nil
	}

	items := make([]RecentFile, 0, len(filePaths))
	now := time.Now().UnixMilli()

	for i, filePath := range filePaths {
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		items = append(items, RecentFile{
			ID:       fileIDFromPath(filePath),
			Name:     info.Name(),
			Path:     filePath,
			Type:     fileTypeFromPath(filePath),
			Date:     info.ModTime().Format("2006-01-02 15:04"),
			Size:     formatFileSize(info.Size()),
			Favorite: false,
			LastUsed: now + int64(i),
		})
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("所选文件不可用")
	}

	return a.AddRecentFiles(items)
}

// ================= 10. OCR 数据结构 =================
type PageInfo struct {
	PageIndex int     `json:"pageIndex"`
	ImagePath string  `json:"imagePath"`
	Blocks    []Block `json:"blocks"`
	Error     string  `json:"error"`
}

type Block struct {
	ClsId         int           `json:"clsId"`
	Label         string        `json:"label"`
	Score         float32       `json:"score"`
	Order         int           `json:"order"`
	Point         []int         `json:"point"`
	Angle         int           `json:"angle"`
	PolygonPoints []image.Point `json:"polygonPoints"`
	Text          string        `json:"text"`
}

// ================= 11. OCR 解析 =================
func (a *App) ParseFile(filePath string) (results []PageInfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ParseFile 发生 Panic: %v", r)
			a.logger.Info("⚠️ [严重错误] %v\n", err)
		}
	}()

	a.logger.Info("\n=== 🚀 后端 ParseFile 被调用: %s ===\n", filePath)

	if _, statErr := os.Stat(filePath); statErr != nil {
		return nil, fmt.Errorf("文件不存在或无法访问: %w", statErr)
	}

	if err := a.ensureSession(); err != nil {
		return nil, err
	}

	a.sessionMu.Lock()
	session := a.paddleOCRVL
	if session != nil {
		session.ApiKey = a.config.ApiKey
		session.Url = a.config.Url
		session.Model = a.config.ModelName
	}
	a.sessionMu.Unlock()

	if session == nil {
		return nil, fmt.Errorf("OCR Session 初始化失败")
	}

	pageResults, err := session.RunOCR(filePath)
	if err != nil {
		a.logger.Error("OCR识别失败: %w", err)
		return nil, fmt.Errorf("OCR识别失败: %w", err)
	}

	results = make([]PageInfo, 0, len(pageResults))
	timeStamp := time.Now().UnixMilli()

	for i, page := range pageResults {
		info := PageInfo{
			PageIndex: page.PageIndex,
		}

		if len(page.Blocks) > 0 {
			info.Blocks = make([]Block, len(page.Blocks))
			for j, b := range page.Blocks {
				info.Blocks[j] = Block{
					ClsId:         b.ClsId,
					Label:         b.Label,
					Score:         b.Score,
					Order:         b.Order,
					Point:         b.Point,
					Angle:         b.Angle,
					PolygonPoints: b.PolygonPoints,
					Text:          b.Text,
				}
			}
		}

		if page.Err != nil {
			info.Error = page.Err.Error()
		}

		if !page.Mat.Empty() {
			fileName := fmt.Sprintf("img_%d_%d.png", timeStamp, i)
			fullPath := filepath.Join(a.tempDir, fileName)

			if success := gocv.IMWrite(fullPath, page.Mat); success {
				info.ImagePath = "/local/" + fileName
			} else {
				info.Error = "保存图片失败"
			}

			page.Mat.Close()
		}

		results = append(results, info)
	}

	// 识别完成后自动持久化，下次直接读取，不重新识别
	if saveErr := a.SaveParseResult(filePath, results); saveErr != nil {
		a.logger.Info("⚠️ 保存识别结果失败: %v\n", saveErr)
	}

	fmt.Println("✅ ParseFile 处理完成")
	return results, nil
}

// ================= 12. 识别结果持久化 =================
func (a *App) cacheDirForFile(filePath string) string {
	id := fileIDFromPath(filePath)
	return filepath.Join(a.cacheDir, id)
}

func (a *App) resultFilePath(filePath string) string {
	return filepath.Join(a.cacheDirForFile(filePath), "result.json")
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func (a *App) GetParseResult(filePath string) ([]PageInfo, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, nil
	}

	data, err := os.ReadFile(a.resultFilePath(filePath))
	if err != nil {
		return nil, nil
	}

	var pages []PageInfo
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, nil
	}

	// 校验图片是否还存在，如果不存在就强制重新识别
	for _, page := range pages {
		if page.ImagePath != "" {
			imgPath := a.resolveImagePath(page.ImagePath)
			if _, err := os.Stat(imgPath); err != nil {
				return nil, nil
			}
		}
	}

	return pages, nil
}

func (a *App) SaveParseResult(filePath string, pages []PageInfo) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("filePath is empty")
	}

	id := fileIDFromPath(filePath)
	dir := filepath.Join(a.cacheDir, id)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 如果图片还在 /local/ 临时目录，就复制到 /cache/ 持久化目录
	for i := range pages {
		imagePath := pages[i].ImagePath

		if strings.HasPrefix(imagePath, "/local/") {
			fileName := filepath.Base(strings.TrimPrefix(imagePath, "/local/"))
			src := filepath.Join(a.tempDir, fileName)
			dstName := fmt.Sprintf("page_%d_%d.png", pages[i].PageIndex, i)
			dst := filepath.Join(dir, dstName)

			if err := copyFile(src, dst); err == nil {
				pages[i].ImagePath = "/cache/" + id + "/" + dstName
			} else {
				a.logger.Info("⚠️ 复制页面图片失败: %v\n", err)
			}
		}
	}

	data, err := json.MarshalIndent(pages, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(a.resultFilePath(filePath), data, 0644)
}

func (a *App) DeleteParseResult(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return nil
	}
	return os.RemoveAll(a.cacheDirForFile(filePath))
}

func (a *App) resolveImagePath(imagePath string) string {
	if strings.HasPrefix(imagePath, "/local/") {
		return filepath.Join(a.tempDir, strings.TrimPrefix(imagePath, "/local/"))
	}

	if strings.HasPrefix(imagePath, "/cache/") {
		return filepath.Join(a.cacheDir, strings.TrimPrefix(imagePath, "/cache/"))
	}

	return filepath.Join(a.tempDir, filepath.Base(imagePath))
}

// ================= 13. 导出数据集结构 =================
type RecognitionResult struct {
	ImageInfo []ImageInfo `json:"image_info"`
	TextInfo  []TextInfo  `json:"text_info"`
}

type ImageInfo struct {
	MatchedTextIndex int    `json:"matched_text_index"`
	ImageURL         string `json:"image_url"`
}

type TextInfo struct {
	Text string `json:"text"`
	Tag  string `json:"tag"`
}

// ================= 14. 导出数据集 =================
func (a *App) ExportData(pageInfos []PageInfo, excludeLabels []string) error {
	if a.ctx == nil {
		return fmt.Errorf("app context is nil")
	}

	if len(pageInfos) == 0 {
		return fmt.Errorf("没有可导出的数据")
	}

	filename, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出数据集 (JSONL)",
		DefaultFilename: fmt.Sprintf("ocr_dataset_%s.jsonl", time.Now().Format("20060102_150405")),
		Filters: []runtime.FileFilter{
			{
				DisplayName: "JSONL Files (*.jsonl)",
				Pattern:     "*.jsonl",
			},
			{
				DisplayName: "All Files (*.*)",
				Pattern:     "*.*",
			},
		},
	})

	if err != nil {
		return err
	}

	if filename == "" {
		return fmt.Errorf("已取消导出")
	}

	dir := filepath.Dir(filename)
	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("创建 assets 目录失败: %w", err)
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("创建导出文件失败: %w", err)
	}
	defer file.Close()

	bufferedWriter := bufio.NewWriter(file)
	defer bufferedWriter.Flush()

	encoder := json.NewEncoder(bufferedWriter)
	encoder.SetEscapeHTML(false)

	excludeMap := make(map[string]bool)
	for _, label := range excludeLabels {
		excludeMap[strings.ToLower(strings.TrimSpace(label))] = true
	}

	for pageIndex, page := range pageInfos {
		if err := a.processPage(pageIndex, page, excludeMap, assetsDir, encoder); err != nil {
			return fmt.Errorf("处理第 %d 页时失败: %w", pageIndex, err)
		}
	}

	_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "导出成功",
		Message: fmt.Sprintf("数据集已成功保存至:\n%s\n(包含同级 assets 文件夹)", filename),
	})

	return nil
}

func (a *App) processPage(
	pageIndex int,
	page PageInfo,
	excludeMap map[string]bool,
	assetsDir string,
	encoder *json.Encoder,
) error {
	if page.ImagePath == "" {
		return nil
	}

	srcImgPath := a.resolveImagePath(page.ImagePath)

	img := gocv.IMRead(srcImgPath, gocv.IMReadColor)
	if img.Empty() {
		a.logger.Info("⚠️ 警告: 无法加载原图 %s，跳过该页\n", srcImgPath)
		return nil
	}
	defer img.Close()

	for blockIndex, block := range page.Blocks {
		label := strings.ToLower(strings.TrimSpace(block.Label))

		if label != "" && excludeMap[label] {
			continue
		}

		var result RecognitionResult
		var maskText, noMaskText string

		switch label {
		case "table":
			maskText = "Table Recognition:"
			otsl, err := utils.ConvertHtmlToOtsl(block.Text)
			if err != nil {
				a.logger.Info("⚠️ 警告: Page %d, Block %d 转换 OTSL 失败: %v，使用原文本降级\n", pageIndex, blockIndex, err)
				noMaskText = block.Text
			} else {
				noMaskText = otsl
			}
		case "formula":
			maskText = "Formula Recognition:"
			noMaskText = block.Text
		case "chart":
			maskText = "Chart Recognition:"
			noMaskText = block.Text
		case "seal":
			maskText = "Seal Recognition:"
			noMaskText = block.Text
		case "spotting":
			maskText = "Spotting:"
			noMaskText = block.Text
		default:
			if excludeMap["text"] || excludeMap["ocr"] {
				continue
			}
			maskText = "OCR:"
			noMaskText = block.Text
		}

		result.TextInfo = append(result.TextInfo,
			TextInfo{Text: maskText, Tag: "mask"},
			TextInfo{Text: noMaskText, Tag: "no_mask"},
		)

		cropMat, err := a.cropBlock(img, block)
		if err != nil {
			a.logger.Info("⚠️ 警告: 裁剪 block 失败 (Page %d, Block %d): %v\n", pageIndex, blockIndex, err)
		} else if cropMat != nil {
			if !cropMat.Empty() {
				uniqueID := uuid.NewString()
				cropFileName := fmt.Sprintf("%s_crop_p%d_b%d.png", uniqueID, pageIndex, blockIndex)
				cropFullPath := filepath.Join(assetsDir, cropFileName)

				if success := gocv.IMWrite(cropFullPath, *cropMat); success {
					result.ImageInfo = append(result.ImageInfo, ImageInfo{
						MatchedTextIndex: 0,
						ImageURL:         "./assets/" + cropFileName,
					})
				} else {
					a.logger.Info("⚠️ 警告: 保存裁剪图片失败 %s\n", cropFullPath)
				}
			}
			cropMat.Close()
		}

		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("序列化并写入 JSONL 失败: %w", err)
		}
	}

	return nil
}

// ================= 15. 裁剪辅助 =================
func (a *App) cropBlock(img gocv.Mat, block Block) (*gocv.Mat, error) {
	point := block.Point

	if len(point) > 4 && len(point)%2 == 0 {
		minX, minY := point[0], point[1]
		maxX, maxY := minX, minY

		for i := 2; i < len(point); i += 2 {
			x := point[i]
			y := point[i+1]
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}

		point = []int{minX, minY, maxX, maxY}
	} else if len(point) < 4 && len(block.PolygonPoints) > 0 {
		minX := block.PolygonPoints[0].X
		minY := block.PolygonPoints[0].Y
		maxX := minX
		maxY := minY

		for _, p := range block.PolygonPoints {
			if p.X < minX {
				minX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}

		point = []int{minX, minY, maxX, maxY}
	}

	if len(point) < 4 {
		return nil, fmt.Errorf("缺少裁剪坐标")
	}

	if point[2] < point[0] {
		point[0], point[2] = point[2], point[0]
	}
	if point[3] < point[1] {
		point[1], point[3] = point[3], point[1]
	}

	layoutDet := &common.LayoutDetResult{Point: point}
	return common.CropByBoxes(layoutDet, &img)
}
