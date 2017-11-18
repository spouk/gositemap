//---------------------------------------------------------------------------
//  генератор карт сайта согласно спецификации протокола
//---------------------------------------------------------------------------
package gositemap

//---------------------------------------------------------------------------
//  IMPORTS
//---------------------------------------------------------------------------
import (
	"fmt"
	"net/url"
	"log"
	"os"
	"time"
	"errors"
	"encoding/xml"
	"strconv"
	"path/filepath"
	"strings"
	"io"
)

//---------------------------------------------------------------------------
//  CONST
//---------------------------------------------------------------------------
const (
	URLSETDEFAULT  = `xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`
	SITEMAPLSCHEMA = `http://www.sitemaps.org/schemas/sitemap/0.9`

	ERRORPRIORITETYRANGE = "ошибка в указании значения приоритета, он должен быть в пределах 0.0 - 1.0"
	ERROREMPTYSTACK      = "стек элементов для формирования карты сайта - пуст, сформируйте его, а потом вызывайте генерацию карты"

	INFOCREATEDSITEMAP = "Успешно создана карта сайта"

	LOGPREFIX = "[gositemap] "
	LOGFLAGS  = log.Ldate | log.Ltime | log.Lshortfile
)

//---------------------------------------------------------------------------
//  TYPES
//---------------------------------------------------------------------------
type (
	Sitemap struct {
		Stock  []*SitemapElement
		Splits map[string][]*SitemapElement
		Logger *log.Logger
		debug  bool
	}
	SitemapElement struct {
		Loc        string
		Changefreq string
		Lastmod    string
		Priority   float64
	}
	changefreqs struct {
		Always  string
		Hourly  string
		Daily   string
		Weekly  string
		Monthly string
		Yearly  string
		Never   string
	}
	//---------------------------------------------------------------------------
	//  SITEMAP XML STRUCT
	//---------------------------------------------------------------------------
	SitemapXML struct {
		XMLName xml.Name      `xml:"urlset"`
		XmlNS   string        `xml:"xmlns,attr"`
		URLS    []*SitemapURL `xml:"url"`
	}
	SitemapURL struct {
		XMLName    xml.Name `xml:"url"`
		Loc        string   `xml:"loc"`
		Lastmod    string   `xml:"lastmod,omitempty"`
		Changefreq string   `xml:"changefreq,omitempty"`
		Priority   float64  `xml:"priority,omitempty"`
	}
)

//создает новый инстанс sitemap
func NewSitemap(logout io.Writer, debug bool) *Sitemap {
	n := &Sitemap{
		Splits: make(map[string][]*SitemapElement),
		debug:  debug,
	}
	if logout != nil {
		n.Logger = log.New(os.Stdout, LOGPREFIX, LOGFLAGS)
	} else {
		n.Logger = log.New(logout, LOGPREFIX, LOGFLAGS)
	}
	return n
}

//создает новый `changefreqs`  - необязательный параметр, указывающий время обновление ссылки `loc`
func (s *Sitemap) NewChangeFreq() *changefreqs {
	c := &changefreqs{
		Always:  "always",
		Daily:   "daily",
		Hourly:  "hourly",
		Monthly: "monthly",
		Never:   "never",
		Weekly:  "weekly",
		Yearly:  "yearly",
	}
	return c
}

//добавляет в общий стек элементов новый элемент для записи в `sitemap.xml`
func (s *Sitemap) NewSitemapElementAdd(loc string, changefreq string, lastmod time.Time, priorit float64) (error) {
	//проверка приоритета
	if priorit < 0.0 || priorit > 1.0 {
		s.Logger.Printf(ERRORPRIORITETYRANGE)
		return errors.New(ERRORPRIORITETYRANGE)
	}
	//пропускаем через фильтр
	if result, err := s.filters(loc); err != nil {
		s.Logger.Printf(err.Error())
		return err
	} else {
		//создаем новый элемент
		res := &SitemapElement{
			Loc:        result,
			Changefreq: changefreq,
			Lastmod:    s.converttime(lastmod),
			Priority:   priorit,
		}
		//добавляем новый элемент
		s.Stock = append(s.Stock, res)
	}
	return nil
}

//символьные фильтры
func (s *Sitemap) filters(str string) (string, error) {
	//определение масок
	mask := make(map[rune]string)
	mask['&'] = `&amp;`
	mask['\''] = `&apos;`
	mask['"'] = `&quot;`
	mask['>'] = `&gt;`
	mask['<'] = `&lt;`
	//parse string filter 1
	result := []rune{}
	for _, x := range str {
		if m, found := mask[x]; found {
			for _, sym := range m {
				result = append(result, sym)
			}
		} else {
			result = append(result, x)
		}
	}
	if u, err := url.Parse(string(result)); err != nil {
		s.Logger.Printf(err.Error())
		return "", err
	} else {
		return u.String(), nil
	}
}

//отображаем стек элементов
func (s *Sitemap) ShowStock() {
	for i, x := range s.Stock {
		fmt.Printf("[%3d] `%#v`\n", i, x)
	}
}



//конвертация времени согласно формату
func (s *Sitemap) converttime(t time.Time) string {
	return t.Format("2006-02-01")
}

//генерация карт сайта
func (s *Sitemap) GenerateXMLSitemap(filenameWithFullPath string) (error) {
	//check stock empty or not
	if len(s.Stock) == 0 {
		s.Logger.Printf(ERROREMPTYSTACK)
		return errors.New(ERROREMPTYSTACK)
	}
	//split stock file segments
	dir, filename, ext := s.SplitFilePath(filenameWithFullPath)
	if s.debug {
		s.Logger.Printf("DIR: `%s` FILENAME: `%s` EXT: `%s`\n", dir, filename, ext)
	}
	s.Splits = s.SplitStock(filename, s.Stock)

	//проверка на наличие количества URL больше 50k
	//работа с сегментами
	for fname, segment := range s.Splits {
		//create new output file
		if s.debug {
			s.Logger.Printf("SEGMENT: fname:`%s`, outfputfilename:`%s`\n", fname, filepath.Join(dir, fname+ext))
		}

		outFile, err := os.Create(filepath.Join(dir, fname+ext))
		if err != nil {
			s.Logger.Printf(err.Error())
			return err
		}
		//make xml
		siteXML := new(SitemapXML)
		siteXML.XmlNS = SITEMAPLSCHEMA

		for _, x := range segment {
			//make new url link
			doc := new(SitemapURL)
			doc.Lastmod = x.Lastmod
			doc.Changefreq = x.Changefreq
			doc.Priority = x.Priority
			doc.Loc = x.Loc
			//append to xml stock
			siteXML.URLS = append(siteXML.URLS, doc)
		}
		resultXML, err := xml.MarshalIndent(siteXML, "", "")
		outXMLString := []byte(xml.Header + string(resultXML))
		if s.debug {
			s.Logger.Printf("OUTXML: %v\v", string(outXMLString))
		}
		//write file result
		countWrite, err := outFile.Write(outXMLString)
		if err != nil {
			s.Logger.Printf(err.Error())
			return err
		}
		if s.debug {
			s.Logger.Printf("[%s] Успешно создана карта сайта размером `%3d` байт\n",
				filepath.Join(dir, fname+ext), countWrite)
		} else {
			s.Logger.Printf(INFOCREATEDSITEMAP)
		}
	}
	return nil
}

//корректор `человеческого` размера
func (s *Sitemap) Sizer(x *SitemapElement) int {
	total := 0
	total += len(x.Loc)
	total += len(strconv.FormatFloat(x.Priority, 'e', -1, 64))
	total += len(x.Changefreq)
	total += len(x.Lastmod)
	return total
}

//тестовый функционал
func (s *Sitemap) GeneratorURL(count int) []*SitemapElement {
	base := "http://testing.loc/page%d.html"
	for x := 0; x < count; x ++ {
		if err := s.NewSitemapElementAdd(
			fmt.Sprintf(base, x),
			s.NewChangeFreq().Daily,
			time.Now(), 0.5); err == nil {
		}

	}
	return s.Stock
}
func (s *Sitemap) SplitStock(filename string, stock []*SitemapElement) map[string][]*SitemapElement {
	//2 критерия: размер и количество ссылок
	result := make(map[string][]*SitemapElement)
	counts := 0
	previos := 0
	size := 0
	x_save := 0
	temp := []*SitemapElement{}
	//проверка на размер в байтах стека элементов
	//если меньше 50k
	if s.debug {
		s.Logger.Printf(">>>>SPLIT RESULT FILENAME: %v\n", filename)
	}
	for x := 0; x < len(stock); x ++ {
		x_save = x
		size += s.Sizer(stock[x])
		//проверка на размер
		if size >= 52428800 {
			//обнуляем счетчик общего размера
			size = 0
			//присваиваем результат в виде среза [previos:x] (предыдущая позиция : текущая )
			//result[counts] = stock[previos:x]
			result[fmt.Sprintf("%s%d", filename, counts)] = temp
			////увеличиваем флаг на точке предыдущего обнуления
			//previos = x
			//увеличиваем общий счетчик сегментов
			counts ++
			//обнуляем temp
			temp = []*SitemapElement{}
		} else {
			//проверка на количество
			if (x - previos) >= 50000 {
				result[fmt.Sprintf("%s%d", filename, counts)] = temp
				//обнуляем temp
				temp = []*SitemapElement{}
				//изменяем текущую позицию
				previos = x
				//увеличиваем общий счетчик сегментов
				counts ++
			} else {
				//размер меньше, все нормально, добавляем во временный сток очередной элемент
				//s.Logger.Printf("[split] count < 50k + temp : %v:%v\n", x, stock)
				temp = append(temp, stock[x])
				if (counts == 0) {
					result[fmt.Sprintf("%s", filename)] = temp
				} else {
					result[fmt.Sprintf("%s%d", filename, counts)] = temp
				}

			}

		}
	}
	//добавляем остаток
	if (x_save-previos) < 50000 && (x_save-previos) > 0 {
		//result[fmt.Sprintf("%s%d", filename,counts)] = temp
	}
	if s.debug {
		s.Logger.Printf(">>>>SPLIT RESULT: %v\n", result)
	}
	return result
}
func (s *Sitemap) SplitFilePath(path string) (dir, filename, ext string) {
	dir, f := filepath.Split(path)
	ext = filepath.Ext(f)
	filename = strings.Split(f, ext)[0]
	return
}
