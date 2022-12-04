package minecraftserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
)

func TestForgeDownloadUrl(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		u, err := forgeDownloadUrl(&v1alpha1.MinecraftServer{
			Spec: v1alpha1.MinecraftServerSpec{
				MinecraftVersion: "1.18.2",
				Forge: &v1alpha1.ForgeSpec{
					ForgeVersion: "40.1.80",
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t,
			"https://maven.minecraftforge.net/net/minecraftforge/forge/1.18.2-40.1.80/forge-1.18.2-40.1.80-installer.jar",
			u.String(),
		)
	})
	t.Run("attempted path escape", func(t *testing.T) {
		u, err := forgeDownloadUrl(&v1alpha1.MinecraftServer{
			Spec: v1alpha1.MinecraftServerSpec{
				MinecraftVersion: "1.18.2",
				Forge: &v1alpha1.ForgeSpec{
					ForgeVersion: "40.1.80/..",
				},
			},
		})
		require.NoError(t, err)
		assert.NotEqual(t,
			"https://maven.minecraftforge.net/net/minecraftforge/forge/1.18.2-40.1.80/../forge-1.18.2-40.1.80/..-installer.jar",
			u.String(),
		)
	})
}
