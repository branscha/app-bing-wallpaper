package main

import (
	"net/http"
	"fmt"
	"os"
	"io/ioutil"
	"encoding/xml"
	"flag"
	"os/user"
	"path"
	"io"
	"os/exec"
	"log"
	"time"
	"sort"
	"strings"
	"net"
)

const (
	bingService        = "www.bing.com:443"
	metaUrlTpl         = "http://www.bing.com/HPImageArchive.aspx?format=xml&idx=%d&n=%d&mkt=%s"
	imgUrlPreferredTpl = "http://www.bing.com%s_%s.jpg"
	imgUrlDefaultTpl   = "http://www.bing.com%s"
	nrImages           = 1
	nrKeeps            = 10
	nrRetries          = 5
	retryDelay         = 10 * time.Second
)

type Image struct {
	Url       string `xml:"url"`
	UrlBase   string `xml:"urlBase"`
	StartDate string `xml:"startdate"`
	EndDate   string `xml:"enddate"`
	Copyright string `xml:"copyright"`
}

type Result struct {
	Images []Image `xml:"image"`
}

var resolutions = map[string]bool{"1024x768":true, "1280x720":true, "1366x768":true, "1920x1080":true, "1920x1200":true}
var picOptions = map[string]bool{"none":true, "wallpaper":true, "centered":true, "scaled":true, "stretched":true, "zoom":true, "spanned":true}
var markets = map[string]bool{"en-US":true, "zh-CN":true, "ja-JP":true, "en-AU":true, "en-UK":true, "de-DE":true, "en-NZ":true, "en-CA":true}

func main() {

	var idx = flag.Uint("index", 0, "0=today, 1=yesterday, ... 7.")
	var res = flag.String("res", "1920x1080", "Preferred resolution 1024x768, 1280x720, 1366x768, 1920x1080, 1920x1200.")
	var picOpt = flag.String("imgOpt", "zoom", "none, wallpaper, centered, scaled, stretched, zoom, spanned")
	var info = flag.Bool("info", false, "Image meta info, no download, no clean.")
	var market = flag.String("market", "en-UK", "en-US, zh-CN, ja-JP, en-AU, en-UK, de-DE, en-NZ, en-CA")

	defaultImageDir := "BingWallPaper"
	usr, err := user.Current()
	if err == nil {
		// We have a home directory, use it to create default image directory.
		defaultImageDir =  path.Join(usr.HomeDir, "Pictures/BingWallpaper")
	}

	var imgDir = flag.String("imgDir", defaultImageDir, "Image directory.")
	var clean = flag.Bool("clean", true, fmt.Sprintf("Keep max. %d images and remove the others.", nrKeeps))

	flag.Usage = usage;

	flag.Parse()

	if len(flag.Args()) > 0 {
		log.Fatal("Unexpected command arguments. Use -help for more information.")
	}

	if _, ok := resolutions[*res]; !ok {
		fmt.Printf("Invalid resolution %s, fall back to default.\n", *res)
		*res = "1920x1080"
	}

	if _, ok := picOptions[*picOpt]; !ok {
		fmt.Printf("Invalid pic option %s, fall back to default.\n", *picOpt)
		*picOpt = "zoom"
	}

	if _, ok := markets[*market]; !ok {
		fmt.Printf("Invalid market %s, fall back to default.\n", *market)
		*market = "en-UK"
	}

	// Verify network availability & Bing availability.
	// This might happen when the utility is scheduled at startup when the network is not yet initialized.

	if err := verifyReachable(bingService, nrRetries, retryDelay); err != nil {
		log.Fatal(err)
	}

	// Get the image meta information from Bing.

	var metaUrl = fmt.Sprintf(metaUrlTpl, *idx, nrImages, *market)
	meta, err := downloadMeta(metaUrl)
	if err != nil {
		log.Fatal(err)
	}

	// Print the meta information.

	start, _ := time.Parse("20060102", meta.Images[0].StartDate)
	startLabel := fmt.Sprintf(start.Format("Jan 2"))

	end, _ := time.Parse("20060102", meta.Images[0].EndDate)
	endLabel := fmt.Sprintf(end.Format("Jan 2"))

	fmt.Println("Title: " + meta.Images[0].Copyright)
	fmt.Println("From: " + startLabel + " until: " + endLabel)

	if *info {
		// We only wanted image info, we are done.
		os.Exit(0)
	}

	// Create the image directory if it doesn't exist.

	if err := os.MkdirAll(*imgDir, os.ModePerm); err != nil {
		log.Fatalf("create image directory: %v", err)
	}

	// Download the image data.

	url1 := fmt.Sprintf(imgUrlPreferredTpl, meta.Images[0].UrlBase, *res)
	url2 := fmt.Sprintf(imgUrlDefaultTpl, meta.Images[0].Url)
	absFileName, err := downloadImage(url1, url2, *imgDir)
	if err != nil {
		log.Fatal(err)
	}

	if err := setDesktop(absFileName, *picOpt); err != nil {
		log.Fatal(err)
	}

	// Cleanup old images.

	if *clean {
		if err := cleanup(*imgDir, nrKeeps); err != nil {
			log.Fatal(err)
		}
	}
}

func usage() {
	fmt.Println(`Download wallpaper images from the Bing website and
install it as the Ubuntu desktop background and lock screen.
You can have the option to choose one of this weeks images.`);
	flag.PrintDefaults();
}

func verifyReachable(service string, maxRetry int, retryDelay time.Duration) (error) {
	for tries := 0; tries < maxRetry; tries++ {
		timeout := time.Duration(1 * time.Second)
		conn, err := net.DialTimeout("tcp", service, timeout)
		if err != nil {
			log.Println("Server unreachable, error: ", err)
			log.Printf("Retry in %v", retryDelay)
		} else {
			conn.Close()
			return nil
		}
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("service %s unreachable", service)
}

// Fetch the image meta information.
// the result is an XML document.
func downloadMeta(metaUrl string) (*Result, error) {
	resp, err := http.Get(metaUrl)
	if err != nil {
		return nil, fmt.Errorf("fetch: %v", err)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch image meta: reading %s: %v\n", metaUrl, err)
	}

	// Unmarshal the image meta information.
	v := Result{}
	err = xml.Unmarshal(b, &v)
	if err != nil {
		return nil, fmt.Errorf("parse image meta: %v", err)
	}
	return &v, nil
}

// Download the Bing wallpaper. First attempt the preferred resolution, and
// if that does not work use the default image.
func downloadImage(url1, url2, imageDir string) (string, error) {
	// First attempt to download the requested resolution.
	imgUrl := url1
	resp, err := http.Get(imgUrl)
	if err != nil {
		// The requested resolution was not available, therefore
		// we fall back to the default images.
		imgUrl = url2
		if resp, err = http.Get(imgUrl); err != nil {
			return "", fmt.Errorf("fetch image data: %v", err)
		}
	}
	defer resp.Body.Close()

	relFileName := path.Base(imgUrl)
	absFileName := path.Join(imageDir, relFileName)

	if _, err := os.Stat(absFileName); os.IsNotExist(err) {
		// path/to/whatever does not exist
		imgFile, err := os.Create(absFileName)
		if err != nil {
			return "", fmt.Errorf("create image file: %v", err)
		}
		defer imgFile.Close()

		if _, err:= io.Copy(imgFile, resp.Body); err != nil {
			return "", fmt.Errorf("copy image: %v", err)
		}
	}
	return absFileName, nil
}

// Change the gnome desktop and lock screen.
func setDesktop(fileName, scaleOptions string) (error) {
	// gsettings set org.gnome.desktop.background picture-uri "file://IMAGE-FILE"
	cmd := "gsettings"
	args := []string{"set", "org.gnome.desktop.background", "picture-uri", "file://" + fileName}
	if err := exec.Command(cmd, args...).Run(); err != nil {
		return fmt.Errorf("set background image: %v", err)
	}

	// gsettings set org.gnome.desktop.background picture-options OPT
	args = []string{"set", "org.gnome.desktop.background", "picture-options", scaleOptions}
	if err := exec.Command(cmd, args...).Run(); err != nil {
		return fmt.Errorf("set background options: %v", err)
	}
	return nil
}

// Cleanup .jpg files in the  directory.
func cleanup(dir string, keeps int) (error) {
	// Fetch the image directory content.
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cleanup images: %v", err)
	}
	// Filter the image files from the directory.
	fileInfos = filter(fileInfos, func (f os.FileInfo) bool {
		return strings.HasSuffix(strings.ToLower(f.Name()), ".jpg")
	})
	// Sort the file info array, oldest images first.
	sort.Slice(fileInfos, func(i, j int) bool { return fileInfos[i].ModTime().Before(fileInfos[j].ModTime()) })
	// Delete the oldest files, keep newest 10.
	if len(fileInfos) > nrKeeps {
		for _, delInfo := range fileInfos[:len(fileInfos) - keeps] {
			if err := os.Remove(path.Join(dir, delInfo.Name())); err != nil {
				// Just give a warning, continue deleting the other files.
				fmt.Printf("cleanup images: %v\n", err)
			} else {
				fmt.Printf("cleanup images: deleted %s\n", delInfo.Name())
			}
		}
	}
	return nil
}

// Only keep jpg file infos in the list. Remove the other file types.
func filter(infos []os.FileInfo, s func(os.FileInfo) bool) (filtered []os.FileInfo) {
	for _, info := range infos {
		if s(info) {
			filtered = append(filtered, info)
		}
	}
	return
}
