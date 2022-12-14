package ctrl

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	gonTplFuncMap           = make(template.FuncMap)
	// gonViewPathTemplates caching map and supported template file ext per view
	gonViewPathTemplates = make(map[string]map[string]*template.Template)
	templatesLock        sync.RWMutex
	gonTemplateFS = defaultFSFunc
	// gonTemplateExt stores the template extension which will build
	gonTemplateExt = []string{"tpl", "html", "gohtml"}
	// gonTemplatePreprocessors stores associations of extension -> preprocessor handler
	gonTemplateEngines = map[string]templatePreProcessor{}
)

func init() {
	gonTplFuncMap["dateformat"] = DateFormat
	gonTplFuncMap["date"] = Date
	gonTplFuncMap["compare"] = Compare
	gonTplFuncMap["compare_not"] = CompareNot
	gonTplFuncMap["not_nil"] = NotNil
	gonTplFuncMap["not_null"] = NotNil
	gonTplFuncMap["substr"] = Substr
	gonTplFuncMap["html2str"] = HTML2str
	gonTplFuncMap["str2html"] = Str2html
	gonTplFuncMap["htmlquote"] = Htmlquote
	gonTplFuncMap["htmlunquote"] = Htmlunquote
	gonTplFuncMap["renderform"] = RenderForm
	gonTplFuncMap["assets_js"] = AssetsJs
	gonTplFuncMap["assets_css"] = AssetsCSS
	// gonTplFuncMap["config"] = GetConfig
	gonTplFuncMap["map_get"] = MapGet

	// Comparisons
	gonTplFuncMap["eq"] = eq // ==
	gonTplFuncMap["ge"] = ge // >=
	gonTplFuncMap["gt"] = gt // >
	gonTplFuncMap["le"] = le // <=
	gonTplFuncMap["lt"] = lt // <
	gonTplFuncMap["ne"] = ne // !=

	gonTplFuncMap["urlfor"] = URLFor // build a URL to match a Controller and it's method
	
	gonViewPathTemplates["views"] = make(map[string]*template.Template)
	buildTemplate("views")
}

type templatePreProcessor func(root, path string, funcs template.FuncMap) (*template.Template, error)

func defaultFSFunc() http.FileSystem {
	return FileSystem{}
}

type templateFile struct {
	root string
	files map[string][]string
}
// visit will make the paths into two part,the first is subDir (without tf.root),the second is full path(without tf.root).
// if tf.root="views" and
// paths is "views/errors/404.html",the subDir will be "errors",the file will be "errors/404.html"
// paths is "views/admin/errors/404.html",the subDir will be "admin/errors",the file will be "admin/errors/404.html"
func (tf *templateFile) visit(paths string, f os.FileInfo, err error) error {
	if f == nil {
		return err
	}
	if f.IsDir() || (f.Mode()&os.ModeSymlink) > 0 {
		return nil
	}
	if !HasTemplateExt(paths) {
		return nil
	}

	replace := strings.NewReplacer("\\", "/")
	file := strings.TrimLeft(replace.Replace(paths[len(tf.root):]), "/")
	subDir := filepath.Dir(file)

	tf.files[subDir] = append(tf.files[subDir], file)
	return nil
}

// HasTemplateExt return this path contains supported template extension of gon or not.
func HasTemplateExt(paths string) bool {
	for _, v := range gonTemplateExt {
		if strings.HasSuffix(paths, "."+v) {
			return true
		}
	}
	return false
}

// BuildTemplate builds all template files in a directory
// enable rendering of any template file in a view directory
func buildTemplate(dir string, files ...string) error {
	var err error
	fs := gonTemplateFS()
	file, err := fs.Open(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.New("dir open err")
	}
	defer file.Close()

	gonTemplates, ok := gonViewPathTemplates[dir]
	if !ok {
		panic("Unknown view path: " + dir)
	}
	self := &templateFile{
		root: dir,
		files: make(map[string][]string),
	}
	err = Walk(fs, dir, func(path string, f os.FileInfo, err error) error {
		return self.visit(path, f, err)
	})
	if err != nil {
		fmt.Printf("Walk() returned %v\n", err)
		return err
	}
	buildAllFiles := len(files) == 0
	for _, v := range self.files {
		for _, file := range v {
			if buildAllFiles || inSlice(file, files) {
				templatesLock.Lock()
				ext := filepath.Ext(file)
				var t *template.Template
				if len(ext) == 0 {
					t, err = getTemplate(self.root, fs, file, v...)
				} else if fn, ok := gonTemplateEngines[ext[1:]]; ok {
					t, err = fn(self.root, file, gonTplFuncMap)
				} else {
					t, err = getTemplate(self.root, fs, file, v...)
				}
				if err != nil {
					log.Println("parse template err:", file, err)
					templatesLock.Unlock()
					return err
				}
				gonTemplates[file] = t
				templatesLock.Unlock()
			}
		}
	}
	return nil
}

// InSlice checks given string in string slice or not.
func inSlice(v string, sl []string) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}

func getTemplate(root string, fs http.FileSystem, file string, others ...string) (t *template.Template, err error) {
	t = template.New(file).Delims("{{", "}}").Funcs(gonTplFuncMap)
	var subMods [][]string
	t, subMods, err = getTplDeep(root, fs, file, "", t)
	if err != nil {
		return nil, err
	}
	t, err = _getTemplate(t, root, fs, subMods, others...)

	if err != nil {
		return nil, err
	}
	return
}

func getTplDeep(root string, fs http.FileSystem, file string, parent string, t *template.Template) (*template.Template, [][]string, error) {
	var fileAbsPath string
	var rParent string
	var err error
	if strings.HasPrefix(file, "../") {
		rParent = filepath.Join(filepath.Dir(parent), file)
		fileAbsPath = filepath.Join(root, filepath.Dir(parent), file)
	} else {
		rParent = file
		fileAbsPath = filepath.Join(root, file)
	}
	f, err := fs.Open(fileAbsPath)
	if err != nil {
		panic("can't find template file:" + file)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, [][]string{}, err
	}
	t, err = t.New(file).Parse(string(data))
	if err != nil {
		return nil, [][]string{}, err
	}
	reg := regexp.MustCompile( "{{" + "[ ]*template[ ]+\"([^\"]+)\"")
	allSub := reg.FindAllStringSubmatch(string(data), -1)
	for _, m := range allSub {
		if len(m) == 2 {
			tl := t.Lookup(m[1])
			if tl != nil {
				continue
			}
			if !HasTemplateExt(m[1]) {
				continue
			}
			_, _, err = getTplDeep(root, fs, m[1], rParent, t)
			if err != nil {
				return nil, [][]string{}, err
			}
		}
	}
	return t, allSub, nil
}

func _getTemplate(t0 *template.Template, root string, fs http.FileSystem, subMods [][]string, others ...string) (t *template.Template, err error) {
	t = t0
	for _, m := range subMods {
		if len(m) == 2 {
			tpl := t.Lookup(m[1])
			if tpl != nil {
				continue
			}
			// first check filename
			for _, otherFile := range others {
				if otherFile == m[1] {
					var subMods1 [][]string
					t, subMods1, err = getTplDeep(root, fs, otherFile, "", t)
					if err != nil {
						log.Println("template parse file err:", err)
					} else if len(subMods1) > 0 {
						t, err = _getTemplate(t, root, fs, subMods1, others...)
					}
					break
				}
			}
			// second check define
			for _, otherFile := range others {
				var data []byte
				fileAbsPath := filepath.Join(root, otherFile)
				f, err := fs.Open(fileAbsPath)
				if err != nil {
					f.Close()
					log.Println("template file parse error, not success open file:", err)
					continue
				}
				data, err = ioutil.ReadAll(f)
				f.Close()
				if err != nil {
					log.Println("template file parse error, not success read file:", err)
					continue
				}
				reg := regexp.MustCompile("{{" + "[ ]*define[ ]+\"([^\"]+)\"")
				allSub := reg.FindAllStringSubmatch(string(data), -1)
				for _, sub := range allSub {
					if len(sub) == 2 && sub[1] == m[1] {
						var subMods1 [][]string
						t, subMods1, err = getTplDeep(root, fs, otherFile, "", t)
						if err != nil {
							log.Println("template parse file err:", err)
						} else if len(subMods1) > 0 {
							t, err = _getTemplate(t, root, fs, subMods1, others...)
							if err != nil {
								log.Println("template parse file err:", err)
							}
						}
						break
					}
				}
			}
		}

	}
	return
}

// ExecuteViewPathTemplate applies the template with name and from specific viewPath to the specified data object,
// writing the output to wr.
// A template will be executed safely in parallel.
func executeViewPathTemplate(wr io.Writer, name string, viewPath string, data interface{}) error {
	if true {//server.GConfig.EnvMode == server.DEV {
		templatesLock.RLock()
		defer templatesLock.RUnlock()
	}
	if gonTemplates, ok := gonViewPathTemplates[viewPath]; ok {
		if t, ok := gonTemplates[name]; ok {
			var err error
			if t.Lookup(name) != nil {
				err = t.ExecuteTemplate(wr, name, data)
			} else {
				err = t.Execute(wr, data)
			}
			if err != nil {
				log.Println("template Execute err:", err)
			}
			return err
		}
		panic("can't find template file in the path:" + viewPath + "/" + name)
	}
	panic("Unknown view path: " + viewPath)
}