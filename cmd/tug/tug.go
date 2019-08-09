package main

import (
  "context"
  "flag"
  "fmt"
  tug "github.com/lijiang2014/tugboat"
  "github.com/lijiang2014/tugboat/docker"
  "github.com/lijiang2014/tugboat/k8s.core"
  "github.com/lijiang2014/tugboat/localos"
  "github.com/lijiang2014/tugboat/storage/local"
  "log"
  "os"
  "path/filepath"
)

var backendType string
var k8sConfig string

var k8sbackend *k8s.Backend
func init() {
  const (
    defaultBackendType = "docker"
    usage         = "backend form [docker local k8s slurm]"
    usageK8sConfig         = "config of k8s config file"
  )
var  defaultk8sConfig =  "/Users/lijiang/workspace/gopher/src/starlight/deployment/config/cluster/power.config"

  flag.StringVar(&backendType, "backend", defaultBackendType, usage)
  flag.StringVar(&backendType, "b", defaultBackendType, usage+"(shorthand)")
  fd, err := os.Open(defaultk8sConfig)
  if err != nil {
    defaultk8sConfig , _ = os.UserHomeDir()
    defaultk8sConfig = filepath.Join(defaultk8sConfig, ".kube/config")
  }
  defer fd.Close()
  flag.StringVar(&k8sConfig, "k", defaultk8sConfig, usageK8sConfig+"(shorthand)")

}

func main() {
  flag.Parse()
  
  ctx := context.Background()
  log := tug.EmptyLogger{}
  store := &local.Local{}
  // 建议 DIR 设置为共享存储
	stage, err := tug.NewStage("tug-workdir", 0755)
  if err != nil {
    panic(err)
  }
  //stage.LeaveDir = true
  lmpexec := "lmp"
  taskid := "test1"
  镜像 := "lammps/lammps"
  //defer stage.RemoveAll()
  var exec tug.Executor
  
  switch backendType {
  case "docker":
    exec = &docker.Docker{
      Logger: log,
    }
  case "local":
    exec = &localos.LocalOS{
      Logger: log,
      EnvAppend: true,
    }
    stage.RDir =  stage.Dir + "/" + taskid
    lmpexec = "lmp_mpi"
  case "k8s":
    if k8sbackend == nil {
      initK8sBackend()
    }
    exec = &k8s.K8sJob{
      Logger: log,
      Namespace: "default",
      ClusterName: "power",
      AppName: "lammps",
      Backend: k8sbackend,
      SetRunAs:true,
      RunAsGid:0,
      RunAsUid:0,
    }
    镜像 = "nscc-gz.cn/lammps"
  default:
    fmt.Println("Unknown backend, use `docker` as backend.")
    exec = &docker.Docker{
      Logger: log,
    }
  }

  task := &tug.Task{
    ID: taskid,
    ContainerImage: 镜像,
    Command: []string{lmpexec, "-in", stage.RDir + "/inputs/indent.in"},
    Stdout: "out.txt",
    Inputs: []tug.File{
      {
        URL: "example/indent.in",
        Path: "/inputs/indent.in",
      },
    },
    Workdir: filepath.Join(stage.Dir, taskid),
    Volumes: []string{ "/tmp" },
    Outputs: []tug.File{
      {
        URL: "output/lammps.out",
        Path: "out.txt",
      },
    },
  }

  err = tug.Run(ctx, task, stage, log, store, exec)
  if err != nil {
    fmt.Println("RESULT", err)
  } else {
    fmt.Println("Success")
  }
}

func initK8sBackend()  {
   k8sbackend = &k8s.Backend{
     Clusters:map[string]k8s.Cluster{
       "power": {
         ConfigPath: k8sConfig,
         //ConfigPath: "/Users/lijiang/workspace/gopher/src/starlight/deployment/config/cluster/power.config",
         //ConfigPath: "/Users/lijiang/workspace/gopher/src/starlight/deployment/config/cluster/power.config",
       },
     },
   }
   if err := k8sbackend.Init() ; err != nil{
     log.Fatalln("k8s backend init failed:\n",err)
   }
   
}