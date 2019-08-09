package tugboat

type RunningTaskController interface {
  Kill(log Logger, store Storage) error
  State() (TaskState, error)
  Wait(log Logger, store Storage) error
  Index() (string)
}

type  EmptyRunningTaskController struct {}

func (*EmptyRunningTaskController) Kill() error {
  return NoSuchTask{}
}

func (*EmptyRunningTaskController) State() (TaskState, error) {
  return TaskUnknown, NoSuchTask{}
}

func (*EmptyRunningTaskController) Wait() error {
  return NoSuchTask{}
}
func (*EmptyRunningTaskController) Index() string {
  return ""
}
