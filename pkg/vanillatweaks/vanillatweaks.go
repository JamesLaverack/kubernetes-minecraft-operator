package vanillatweaks

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/json"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/version"
)

func GetDatapackDownloadURL(ctx context.Context, v string, datapacks []minecraftv1alpha1.VanillaTweaksDatapack) (string, error) {
	log := logutil.FromContextOrNew(ctx)

	selected := make(map[string][]string)
	for _, d := range datapacks {
		selected[d.Category] = append(selected[d.Category], d.Name)
	}
	selectedEncoded, err := json.Marshal(selected)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Add("version", version.ParseMinorVersion(v))
	form.Add("packs", string(selectedEncoded))

	resp, err := http.PostForm("https://vanillatweaks.net/assets/server/zipdatapacks.php", form)
	if err != nil {
		return "", err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	log.With(
		zap.String("request", form.Encode()),
		zap.String("response", string(data))).
		Debug("Made request to Vanilla Tweaks API")

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		return "", err
	}

	status, ok := parsed["status"]
	if !ok {
		return "", errors.New("Response from VanillaTweaks did not have status key")
	}
	statusString, ok := status.(string)
	if !ok {
		return "", errors.New("status key in VanillaTweaks response was not a string")
	}
	if statusString != "success" {
		return "", errors.New("status from VanillaTweaks was not 'success'")
	}

	link, ok := parsed["link"]
	if !ok {
		return "", errors.New("Response from VanillaTweaks did not have link key")
	}
	linkString, ok := link.(string)
	if !ok {
		return "", errors.New("link key in VanillaTweaks response was not a string")
	}

	return "https://vanillatweaks.net" + linkString, nil
}
