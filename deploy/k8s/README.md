# P03 Kubernetes manifests

api / web / processor。環境変数の本番値（DB URL、OIDC）は `pf-cloud-k8s` overlay の patch で上書きする。

```powershell
cd ..\..\pf-cloud-k8s
.\scripts\build-images.ps1
.\scripts\up.ps1
```

Compose 単体デモは従来どおり `deploy/compose.yaml`。
