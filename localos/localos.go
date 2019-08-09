package localos

/*
  local os 只用来进行本地开发调试用，不用于生产系统
  只考虑单实例部署的场景
*/

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	
	tug "github.com/lijiang2014/tugboat"
)

type LocalOS struct {
	tug.Logger
	EnvAppend bool
}

var runtimeUuidMap = map[string]*runtimeLocal{}
var runtimePidMap = map[int]*runtimeLocal{}

type runtimeLocal struct {
	Pid int
	*exec.Cmd
	cancel context.CancelFunc
	Task   *tug.StagedTask
	ctx    context.Context
	//task *tug.StagedTask,
	stdio *tug.Stdio
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
		if _, errt := os.Stat(host); errt != nil {
			if err := os.MkdirAll(host, 0755); err != nil {
				return err
			}
		}
		d.Meta("volumes ", host+" "+c)
	}
	var err error

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf(`exec start failed: %s`, err)
	}

	d.Meta("pid", cmd.Process.Pid)

	return cmd.Wait()
}

func (d *LocalOS) Start(ctx context.Context, task *tug.StagedTask, stdio *tug.Stdio) (jobctl tug.RunningTaskController, err error) {
	args := []string{}
	envs := []string{}
	if d.EnvAppend {
		envs = os.Environ()
	}
	for k, v := range task.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, task.Command...)

	d.Meta("command", strings.Join(args, " "))

	cmdctx, cancel := context.WithCancel(ctx)
	//defer cancel()
	cmd := exec.CommandContext(cmdctx, args[0], args[1:]...)
	cmd.Stdin = stdio.Stdin
	cmd.Stdout = stdio.Stdout
	cmd.Stderr = stdio.Stderr

	for i, host := range task.Volumes {
		c := task.Task.Volumes[i]
		if c == task.Workdir {
			cmd.Dir = host
		} else {
			// 建立文件夹软链接
			// to-do
		}
		if _, errt := os.Stat(host); errt != nil {
			if err = os.MkdirAll(host, 0755); err != nil {
				return
			}
		}
		d.Meta("volumes ", host+" "+c)
	}

	err = cmd.Start()
	//log.Println("start err", err)
	if err != nil {
		err = fmt.Errorf(`exec start failed: %s`, err)
		return
	}
	if cmd.Process == nil {
		err = fmt.Errorf(`exec start failed with cannot start.`)
		return
	}

	d.Meta("pid", cmd.Process.Pid)

	commit := &runtimeLocal{
		Cmd:    cmd,
		cancel: cancel,
		stdio:  stdio,
		Task:   task,
	}
	//return cmd.Wait()
	runtimePidMap[cmd.Process.Pid] = commit
	runtimeUuidMap[task.ID] = commit
	return commit, nil
	//return cmd.Process.Pid, nil
	//return 0, nil
}

func (d *LocalOS) State(ctx context.Context, task *tug.StagedTask, stdio *tug.Stdio, jobid int) (err error) {
	if runtimel, ok := runtimeUuidMap[task.ID]; ok {
		ps := runtimel.Cmd.ProcessState
		if ps == nil {
			task.State = tug.TaskRunning
			return
		}
		if ps.Exited() {
			if ps.Success() {
				task.State = tug.TaskSuccess
			} else {
				task.State = tug.TaskFailed
			}
			return
		}
		task.State = tug.TaskRunning
		return
	}
	err = tug.NoSuchTask{task.ID, jobid}
	return
}

func (rl *runtimeLocal) State() (s tug.TaskState, err error) {
	ps := rl.Cmd.ProcessState
	if ps == nil {
		return tug.TaskRunning, nil

	}
	if ps.Exited() {
		if ps.Success() {
			return tug.TaskSuccess, nil
		} else {
			return tug.TaskFailed, nil
		}
	}
	return tug.TaskRunning, nil
}

func (d *LocalOS) Kill(ctx context.Context, task *tug.StagedTask, stdio *tug.Stdio, jobid int) (err error) {
	if runtimel, ok := runtimeUuidMap[task.ID]; ok {
		//return runtimel.Cmd.Wait()
		runtimel.cancel()

		return d.State(ctx, task, stdio, jobid)
	}
	err = tug.NoSuchTask{task.ID, jobid}
	return
}

func (rl *runtimeLocal) Kill(log tug.Logger, store tug.Storage) (err error) {
	rl.cancel()
	// CHANNEL TIMEOUT
	rl.Wait(log, store)
	_, err = rl.State()
	return
}

func (d *LocalOS) Wait(ctx context.Context, task *tug.StagedTask, stdio *tug.Stdio, jobid int) (err error) {
	if runtimel, ok := runtimeUuidMap[task.ID]; ok {
		return runtimel.Cmd.Wait()
	}
	err = tug.NoSuchTask{task.ID, jobid}
	return
}

func (rl *runtimeLocal) Wait(log tug.Logger, store tug.Storage) (err error) {
	// 添加通用的资源回收逻辑
	var me tug.MultiError
	try := me.Try
	defer func() {
		fmt.Println("\nDebug finish\n")
		err = me.Finish()
	}()
	defer func() {
		fmt.Println("\nDebug removeall\n")
		try(rl.Task.RemoveAll())
	}()
	defer func() {
		fmt.Println("\nDebug upload\n")
		try(tug.Upload(rl.ctx, rl.Task, store, log))
	}()
	defer func() {
		try(rl.stdio.Close())
	}()

	return rl.Cmd.Wait()
}

func (rl *runtimeLocal) Index() string {
	return fmt.Sprint(rl.Cmd.Process.Pid)
}

func (d *LocalOS) RecoverRunningTaskController(t *tug.StagedTask, index string) (tug.RunningTaskController, error) {
	return nil, fmt.Errorf("Cannot recover local runtime controler")
}
