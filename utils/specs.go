package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/AnishDe12020/spintron"
	"google.golang.org/api/option"
)

type SpecTemplate struct {
	Name        string                   `json:"name"`
	StaticFiles []SpecTemplateStaticFile `json:"staticFiles"`
	Questions   []SpecsTemplateQuestion  `json:"questions"`
}

type SpecsTemplateQuestion struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Default string `json:"default"`
}

type SpecTemplateStaticFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

func GetTemplates() ([]string, error) {
	templates := []string{}
	starliSpecsDir := GetStarliSpecsCacheDir()
	matches, err := filepath.Glob(starliSpecsDir + "/templates/**/starli.json")

	if err != nil {
		return nil, err
	}

	for _, path := range matches {
		templateData, err := ioutil.ReadFile(path)

		if err != nil {
			return nil, err
		}

		var template SpecTemplate

		err = json.Unmarshal(templateData, &template)

		if err != nil {
			return nil, err
		}

		templates = append(templates, template.Name)
	}

	return templates, nil
}

// func GetTemplate(name string) (*tmpl.Template, error) {
// 	starliSpecsDir := GetStarliSpecsCacheDir()

// 	matches, _ := filepathx.Glob(starliSpecsDir + "/templates/" + strings.ToLower(name) + "/**/*.tmpl")

// 	templates, err := tmpl.ParseFiles(matches...)

// 	if err != nil {
// 		fmt.Println(err)
// 	}

// 	return templates, nil
// }

func GetTemplate(name string) (SpecTemplate, error) {
	starliSpecsDir := GetStarliSpecsCacheDir()
	// Get Starli.json file of template and return the json value
	var template SpecTemplate
	starliSpecsFile := starliSpecsDir + "/templates/" + strings.ToLower(name) + "/starli.json"
	templateData, err := ioutil.ReadFile(starliSpecsFile)

	if err != nil {
		return template, err
	}

	err = json.Unmarshal(templateData, &template)

	if err != nil {
		return template, err
	}

	return template, nil
}

func CheckIfSpecsExists() (bool, error) {
	starliSpecsDir := GetStarliSpecsCacheDir()
	specsEtagFile := GetStarliSpecsEtagFile()

	if _, err := os.Stat(specsEtagFile); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if _, err := os.Stat(starliSpecsDir); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func DownloadSpecsDir() error {
	s := spintron.New(spintron.Options{
		Text: "Downloading Starli specs...",
	})
	s.Start()

	starliDirPath := GetStarliCacheDir()

	if _, err := os.Stat(starliDirPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(starliDirPath, os.ModePerm)
		if err != nil {
			s.Fail("Failed to create starli directory")
			return err
		}
	}

	ctx := context.Background()

	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		s.Fail("Failed to initialize a Google Cloud Storage client")
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	rc, err := client.Bucket("starli-cli.appspot.com").Object("specs.tar").NewReader(ctx)
	if err != nil {
		s.Fail("Failed to download Starli specs")
		return err
	}
	defer rc.Close()

	err = Untar(starliDirPath, rc)
	if err != nil {
		s.Fail("Failed to untar Starli specs")
		return err
	}

	attrs, err := client.Bucket("starli-cli.appspot.com").Object("specs.tar").Attrs(ctx)
	if err != nil {
		s.Fail("Failed to get Starli specs attributes")
		return err
	}

	starliSpecsEtagPath := GetStarliSpecsEtagFile()

	err = os.WriteFile(starliSpecsEtagPath, []byte(attrs.Etag), 0644)
	if err != nil {
		s.Fail("Failed to write Starli specs etag")
		return err
	}

	s.Succeed("Specs downloaded")

	return nil

}

func UpdateSpecs(isVerbose bool) error {
	s := spintron.New(spintron.Options{
		Text: "Updating Starli specs...",
	})

	if isVerbose {
		s.Start()
	}

	starliDirPath := GetStarliCacheDir()

	if _, err := os.Stat(starliDirPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(starliDirPath, os.ModePerm)
		if err != nil {
			if isVerbose {
				s.Fail("Failed to create starli directory")
			}
			return err
		}
	}

	ctx := context.Background()

	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		if isVerbose {
			s.Fail("Failed to initialize a Google Cloud Storage client")
		}
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	attrs, err := client.Bucket("starli-cli.appspot.com").Object("specs.tar").Attrs(ctx)
	if err != nil {
		if isVerbose {
			s.Fail("Failed to get Starli specs attributes")
		}
		return err
	}

	starliSpecsEtagPath := GetStarliSpecsEtagFile()

	existingEtag, err := os.ReadFile(starliSpecsEtagPath)
	if err != nil {
		if isVerbose {
			s.Fail("Failed to read Starli specs etag")
		}
		return err
	}

	if string(existingEtag) == attrs.Etag {
		if isVerbose {
			s.Succeed("Specs up to date")
		}
		return nil
	}

	rc, err := client.Bucket("starli-cli.appspot.com").Object("specs.tar").NewReader(ctx)
	if err != nil {
		if isVerbose {
			s.Fail("Failed to download Starli specs")
		}
		return err
	}
	defer rc.Close()

	err = Untar(starliDirPath, rc)

	if err != nil {
		if isVerbose {
			s.Fail("Failed to untar Starli specs")
		}
		return err
	}

	err = os.WriteFile(starliSpecsEtagPath, []byte(attrs.Etag), 0644)
	if err != nil {
		if isVerbose {
			s.Fail("Failed to write Starli specs etag")
		}
		return err
	}

	if isVerbose {
		s.Succeed("Specs updated")
	}

	return nil
}

func DeleteSpecs() error {
	starliDirPath := GetStarliSpecsCacheDir()

	err := os.RemoveAll(starliDirPath)
	if err != nil {
		ErrorPrint("Failed to delete Starli specs")
		return err
	}

	starliSpecsEtagPath := GetStarliSpecsEtagFile()

	err = os.Remove(starliSpecsEtagPath)
	if err != nil {
		ErrorPrint("Failed to delete Starli specs etag")
		fmt.Println(err)
		return err
	}

	Success("Specs deleted")

	return nil
}

func RemoveStarliSpecsConfigPathForFile(path string, template string) string {
	starliSpecsDir := GetStarliSpecsCacheDir()
	newPath := strings.Replace(path, starliSpecsDir+"/templates/"+strings.ToLower(template)+"/", "", 1)
	fmt.Println(path)
	fmt.Println(template)
	fmt.Println(newPath)
	return newPath
}

func RemoveStarliSpecsConfigPathForDir(path string, template string) string {
	starliSpecsDir := GetStarliSpecsCacheDir()
	newPath := strings.Replace(path, starliSpecsDir+"/templates/"+strings.ToLower(template), "", 1)
	fmt.Println(path)
	fmt.Println(template)
	fmt.Println(newPath)
	return newPath
}
