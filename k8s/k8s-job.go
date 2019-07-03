package k8s

import (
  "context"
  "fmt"
  tug "github.com/lijiang2014/tugboat"
  "io/ioutil"
  "os"
  "os/exec"
  "strings"
)


type K8sJob struct {
  tug.Logger
  EnvAppend bool
  Namespace string
}


func (d *K8sJob) Exec(ctx context.Context, task *tug.StagedTask, stdio *tug.Stdio) error {
  var (
    appname = "appname"
    // appImg = "appImg"
    appCommand = ""
  )
  // set appCommand
  for i, comargi := range task.Command {
    if i != 0 {
      appCommand += "\n"
    }
    appCommand += jobTempCommandItemIndent + comargi
  }
  
  var jobyaml string
  jobyaml = jobTem
  jobyaml = strings.ReplaceAll(jobyaml, "$(namespace)",d.Namespace)
  jobyaml = strings.ReplaceAll(jobyaml, "$(app.name)",appname)
  jobyaml = strings.ReplaceAll(jobyaml, "$(job.name)",task.ID)
  jobyaml = strings.ReplaceAll(jobyaml, "$(app.image)",task.ContainerImage)
  jobyaml = strings.ReplaceAll(jobyaml, jobTempCommandItemIndent +"$(app.command)",appCommand)
  
  
  // todo set vols 设置挂载的存储
  // todo
  
  file ,err := ioutil.TempFile(os.TempDir(), "k8sjob-*.yaml")
  defer file.Close()
  if err != nil {
    return err
  }
  d.Meta(file.Name(), jobyaml)
  //d.Meta("container name", name)
  
  //cmd := exec.Command(args[0], args[1:]...)
  cmdctx, cancel := context.WithCancel(ctx)
  defer cancel()
  cmd := exec.CommandContext(cmdctx, "kubectel" , "create", "-f", file.Name())
  cmd.Stdin = stdio.Stdin
  cmd.Stdout = stdio.Stdout
  cmd.Stderr = stdio.Stderr
  
  //cmd.Dir = task.Workdir
  for i, host := range task.Volumes {
    c := task.Task.Volumes[i]
    if c == task.Workdir {
      cmd.Dir = host
    } else {
      // 建立文件夹软链接
      // to-do
    }
    d.Meta("volumes ", host + " "+ c)
  }
  
  err = cmd.Start()
  if err != nil {
    return fmt.Errorf(`exec start failed: %s`, err)
  }
  
  d.Meta("pid", cmd.Process.Pid)
  
  
  return cmd.Wait()
}