package bibliothek

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type VersionResponse struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	Version     string `json:"version"`
	Builds      []int  `json:"builds"`
}

func LatestBuildForVersion(version string) (int, error) {
	r, err := http.Get("https://api.papermc.io/v2/projects/paper/versions/" + version)
	if err != nil {
		return -1, err
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return -1, err
	}

	var ver VersionResponse
	err = json.Unmarshal(body, &ver)
	if err != nil {
		return -1, err
	}

	if len(ver.Builds) == 0 {
		return -1, errors.New("no builds for version")
	}

	latestBuild := ver.Builds[0]
	for _, build := range ver.Builds {
		if build > latestBuild {
			latestBuild = build
		}
	}
	return latestBuild, nil
}

type DownloadsResponse struct {
	ProjectID   string              `json:"project_id"`
	ProjectName string              `json:"project_name"`
	Version     string              `json:"version"`
	Build       int                 `json:"build"`
	Time        string              `json:"time"`
	Channel     string              `json:"channel"`
	Promoted    bool                `json:"promoted"`
	Downloads   map[string]Download `json:"downloads"`
}

type Download struct {
	Name   string `json:"name"`
	Sha256 string `json:"sha256"`
}

func GetDownloadURLAndSHA256(version string, build int) (string, string, error) {
	r, err := http.Get(fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s/builds/%d", version, build))
	if err != nil {
		return "", "", err
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", "", err
	}

	var down DownloadsResponse
	err = json.Unmarshal(body, &down)
	if err != nil {
		return "", "", err
	}

	appDownload, ok := down.Downloads["application"]
	if !ok {
		return "", "", errors.New("unable to find application download for version")
	}

	return fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s/builds/%d/downloads/%s", version, build, appDownload.Name), appDownload.Sha256, nil
}
