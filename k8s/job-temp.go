package k8s

const jobTem = `apiVersion: batch/v1
kind: Job
namespace: $(namespace)
metadata:
  labels:
    app: $(app.name)
  name: $(job.name)
spec:
  template:
    metadata:
      name: $(job.name)
    spec:
      containers:
        - name: $(job.name)
          image: $(app.image)
          command:
          - $(app.command)
      restartPolicy: Never
`
const jobTempCommandItemIndent = `          - `

// 其他待补充的内容
// input files
// mounted volumes
// Env ?
// Endpoints ?

// models
// starlight-app => starlight.job --> pods.