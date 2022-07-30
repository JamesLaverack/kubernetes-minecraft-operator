package vanillatweaks

import (
	"fmt"
	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetDatapackDownloadURL(t *testing.T) {
	t.Run("multiple packs", func(t *testing.T) {
		url, err := getDatapackDownloadURL("1.19", []minecraftv1alpha1.VanillaTweaksDatapack{
			{
				Name: "real time clock",
				Category: "survival",
			},
			{
				Name: "silence mobs",
				Category: "mobs",
			},
		})
		require.NoError(t, err)
		fmt.Println(url)
		require.NotEmpty(t, url)
	})
}