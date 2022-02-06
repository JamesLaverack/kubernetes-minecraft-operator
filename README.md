# Minecraft Operator

A Kubernetes [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) for dedicated servers of the
video game [Minecraft Java Edition](https://www.minecraft.net/en-us/store/minecraft-java-edition).
This allows you to administer and configure a Minecraft server solely using the Kubernetes API — no remote shell, SFTP,
or graphical admin interfaces are required.

Note that this is 🚧 alpha-grade 🚧 software. No guarantee is provided (see the [license](LICENSE)), and users are
responsible for the security and data-integrity of their own servers. The API surface is subject to change. This
software is not endorsed or supported by Microsoft, Mojang, or the Minecraft development team.

All the heavy-lifting is done by the excellent [itzg/docker-minecraft-server](https://github.com/itzg/docker-minecraft-server).
That container image is what is actually deployed into the cluster as a `Pod`. This project isn't endorsed or
supported by itzg either.

## Installing

You can install the latest version of the operator by running this command with your cluster configured in `kubectl`.

```bash
$ curl -L https://github.com/JamesLaverack/minecraft-operator/releases/latest/download/operator.yaml | kubectl apply -f -
```

### Tags

The command above will install the operator at a specific release. You can also change the image tag to either `latest`
to always get the latest published release or `edge` to get the latest build of the main branch. However, doing so won't
get you YAML updates (e.g., adding new permissions or custom resource definition updates). (Also the `edge` tag
publishes *before* tests are run, so it comes with even fewer guarantees than normal.)

### Verify

You can verify everything is working by looking for the operator Pod in the `minecraft-operator-system` namespace, e.g.:

```bash
$ kubectl --namespace minecraft-operator-system get pod
NAME                                                     READY   STATUS    RESTARTS   AGE
minecraft-operator-controller-manager-69f45b8b85-42fsh   2/2     Running   0          7m47s
```

## Usage

Once the operator is installed, you can create a Minecraft server by creating a `MinecraftServer` object in Kubernetes.
This will create and manage resources such as a Pod and a Service. Resources are created in whatever namespace the
`MinecraftServer` object is in.

Note that the API uses "allowList" in place of "whitelist", but it is applied to the server in the same way.

### Example Server

```yaml
apiVersion: minecraft.jameslaverack.com/v1alpha1
kind: MinecraftServer
metadata:
  name: my-minecraft-server
spec:
  eula: Accepted
  minecraftVersion: 1.18.1
  type: Paper
  opsList:
    - name: Player1
      uuid: da6a1ae6-e2f5-4e32-9135-b82a9ef426a9
  allowList:
    - name: Player2
      uuid: 880182d6-a0e3-44cd-a57c-8dc3799e92b8
  world:
    persistentVolumeClaim:
      claimName: minecraft-world
  motd: "My Minecraft Server"
  maxPlayers: 8
  viewDistance: 16
  externalServiceIP: 192.168.1.51
  vanillaTweaks:
    survival:
      - 'multiplayer sleep'
      - 'afk display'
    items:
      - 'player head drops'
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: minecraft-world
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```
