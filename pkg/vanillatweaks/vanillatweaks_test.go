package vanillatweaks

import (
	"context"
	"fmt"
	"testing"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestGetDatapackDownloadURL(t *testing.T) {
	t.Run("multiple packs", func(t *testing.T) {
		url, err := GetDatapackDownloadURL(context.Background(), "1.19", []minecraftv1alpha1.VanillaTweaksDatapack{
			{
				Name:     "real time clock",
				Category: "survival",
			},
			{
				Name:     "silence mobs",
				Category: "mobs",
			},
		})
		require.NoError(t, err)
		fmt.Println(url)
		require.NotEmpty(t, url)
	})
}
