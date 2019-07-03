package localos


import (
  "context"
  "fmt"
  tug "github.com/lijiang2014/tugboat"
  "os"
  "os/exec"
  "strings"
)


type LocalOS struct {
  tug.Logger
  EnvAppend bool
}


func (d *LocalOS) Exec(ctx context.Context, task *tug.StagedTask, stdio *tug.Stdio) error {
  
  
  args := []string{}
  envs := []string{}
  if d.EnvAppend {
    envs = os.Environ()
  }
  for k, v := range task.Env {
    envs = append(envs, fmt.Sprintf("%s=%s", k, v))
  }
  
  //name := fmt.Sprintf("task-%s-%s", task.ID, randString(5))
  //args = append(args, "--name", name)
  
  //for i, input := range task.Inputs {
  //  host := input.Path
  //  container := task.Task.Inputs[i].Path
  //  arg := formatVolumeArg(host, container, true)
  //  args = append(args, "-v", arg)
  //}
  
  //for i, host := range task.Volumes {
  //  container := task.Task.Volumes[i]
  //  arg := formatVolumeArg(host, container, false)
  //  args = append(args, "-v", arg)
  //}
  
  //args = append(args, task.ContainerImage)
  args = append(args, task.Command...)
  
  // Roughly: `docker run --rm -i --read-only -w [workdir] -v [bindings] [imageName] [cmd]`
  d.Meta("command", strings.Join(args, " "))
  //d.Meta("container name", name)
  
  //cmd := exec.Command(args[0], args[1:]...)
  cmdctx, cancel := context.WithCancel(ctx)
  defer cancel()
  cmd := exec.CommandContext(cmdctx, args[0], args[1:]...)
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
  var err error
  
  err = cmd.Start()
  if err != nil {
    return fmt.Errorf(`exec start failed: %s`, err)
  }
  
  d.Meta("pid", cmd.Process.Pid)
  
  
  return cmd.Wait()
}