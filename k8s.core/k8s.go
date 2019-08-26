package k8s

// 通过调用 K8S API 去实现 Executor FOR K8S JOB

import (
  "context"
  "errors"
  "fmt"
  tug "github.com/lijiang2014/tugboat"
  "io"
  "io/ioutil"
  appv1 "k8s.io/api/apps/v1"
  v1 "k8s.io/api/batch/v1"
  corev1 "k8s.io/api/core/v1"
  "k8s.io/apimachinery/pkg/util/intstr"
  "k8s.io/client-go/kubernetes"
  restclient "k8s.io/client-go/rest"
  "k8s.io/client-go/tools/clientcmd"
  "log"
  "os"
  
  "sigs.k8s.io/yaml"
  //"k8s.io/kubernetes/pkg/kubectl/cmd/attach"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  
  "time"
)

type Backend struct {
  Clusters map[string]Cluster
}

type K8sJob struct {
  tug.Logger
  EnvAppend   bool
  Namespace   string
  ClusterName string
  AppName     string
  SetRunAs    bool
  RunAsUid    int64
  RunAsGid    int64
  Endpoints   []Endpoint
  Type        K8sJobType
  tug.RuntimeParams
  *Backend
}

type K8sJobType string

const (
  K8sJobTypeBatchJob   K8sJobType = "Job"
  K8sJobTypeDeployment K8sJobType = "Deployment"
)

type Endpoint struct {
  Name       string
  Port       int
  TargetPort int
}

type Cluster struct {
  ConfigPath string // 配置文件路径
  config     *restclient.Config
  client     *kubernetes.Clientset
}

type K8sJobCtl struct {
  Job       string
  Cluster   string
  Namespace string
}

func (d *Backend) Init() error {
  // 初始化 CMDCLIENT
  for clusterName, clusteri := range d.Clusters {
    
    k8sConfig, err := clientcmd.BuildConfigFromFlags("", clusteri.ConfigPath)
    if err != nil {
      return err
    }
    k8sConfig.Timeout = 3 * time.Second
    clusteri.config = k8sConfig
    k8sClientSet, err := kubernetes.NewForConfig(k8sConfig)
    if err != nil {
      return err
    }
    clusteri.client = k8sClientSet
    d.Clusters[clusterName] = clusteri
    
  }
  return nil
}

func (d *K8sJob) Exec(ctx context.Context, task *tug.StagedTask, stdio *tug.Stdio) error {
  
  // set appCommand
  var client *kubernetes.Clientset
  if c, got := d.Backend.Clusters[d.ClusterName]; got {
    client = c.client
  } else {
    return errors.New("Bad Cluster Name")
  }
  commands := task.Command
  podLabelsSet := map[string]string{"app": d.AppName, "name": task.ID}
  podSelector := map[string]string{"name": task.ID}
  var ttlSecondsAfterFinished int32 = 3600 * 24
  var ttlSecondsAfterFinishedPt = &ttlSecondsAfterFinished
  /*
     经测试 ttlSecondsAfterFinished 机制在 POWER 集群上未生效，根据文档，应该是相关 K8S 特性未启用
     https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/
  */
  envs := []corev1.EnvVar{}
  requests := make(corev1.ResourceList)
  requests[corev1.ResourceCPU] = d.RuntimeParams.CPU()
  //requests[corev1.ResourceGPU] = d.RuntimeParams.GPU()
  requests[corev1.ResourceMemory] = d.RuntimeParams.Memory()
  volumeMounts := make([]corev1.VolumeMount, 0)
  volumes := make([]corev1.Volume, 0)
  securityContext := &corev1.SecurityContext{
    Capabilities: &corev1.Capabilities{
      Add: []corev1.Capability{
        "SYS_ADMIN",
      },
    },
  }
  var podSecurityContext *corev1.PodSecurityContext
  if d.SetRunAs {
    podSecurityContext = &corev1.PodSecurityContext{
      RunAsUser:  &d.RunAsUid,
      RunAsGroup: &d.RunAsGid,
    }
  }
  // TO-DO
  // Copy inputs into remote fs.
  var workdirMounted bool
  for i, input := range task.Inputs {
    // host := filepath.Join( "/nfs_data" ,input.Path)
    host := input.Path
    container := task.Task.Inputs[i].Path
    volumeiName := "input" + fmt.Sprint(i)
    volumei := corev1.Volume{
      Name: volumeiName,
      VolumeSource: corev1.VolumeSource{
        HostPath: &corev1.HostPathVolumeSource{
          Path: host,
        },
      },
    }
    volumeMounti := corev1.VolumeMount{
      Name:      volumeiName,
      ReadOnly:  true,
      MountPath: container,
    }
    volumeMounts = append(volumeMounts, volumeMounti)
    volumes = append(volumes, volumei)
    if host == task.Workdir {
      workdirMounted = true
    }
  }
  
  // mount Workdir
  if task.Workdir != "" && !workdirMounted {
    host := task.Workdir
    container := task.Task.Workdir
    volumeiName := "workdir"
    volumei := corev1.Volume{
      Name: volumeiName,
      VolumeSource: corev1.VolumeSource{
        HostPath: &corev1.HostPathVolumeSource{
          Path: host,
        },
      },
    }
    volumeMounti := corev1.VolumeMount{
      Name:      volumeiName,
      MountPath: container,
    }
    volumeMounts = append(volumeMounts, volumeMounti)
    volumes = append(volumes, volumei)
  }
  
  targetJob := &v1.Job{
    ObjectMeta: metav1.ObjectMeta{
      Name: task.ID,
    },
    Spec: v1.JobSpec{
      TTLSecondsAfterFinished: ttlSecondsAfterFinishedPt,
      // Selector: &metav1.LabelSelector{MatchLabels: podSelector},
      Template: corev1.PodTemplateSpec{
        ObjectMeta: metav1.ObjectMeta{
          Labels: podLabelsSet,
        },
        
        Spec: corev1.PodSpec{
          //NodeSelector: map[string]string{ "name":"power20" },
          Containers: []corev1.Container{
            {
              Name:  task.ID,
              Image: task.ContainerImage,
              //ImagePullPolicy: "Always",
              ImagePullPolicy: "Never",
              Command:         commands,
              Env:             envs,
              WorkingDir:      task.Task.Workdir,
              Resources: corev1.ResourceRequirements{
                Requests: requests,
                Limits:   requests,
              },
              VolumeMounts:    volumeMounts,
              SecurityContext: securityContext,
            },
          }, // containers end
          Volumes:         volumes,
          SecurityContext: podSecurityContext,
          RestartPolicy:   "Never", // supported values: "OnFailure", "Never"
        },
      },
    },
  }
  jobyaml, _ := yaml.Marshal(targetJob)
  log.Println("job.yaml:\n", string(jobyaml))
  
  jobcreated, err := client.BatchV1().Jobs(d.Namespace).Create(targetJob)
  if err != nil {
    log.Println("create job failed:", jobcreated, err)
    return err
  }
  d.Meta("job", jobcreated)
  defer func() {
    // DEFER 删除 JOB
    propagation := metav1.DeletePropagationBackground
    client.BatchV1().Jobs(d.Namespace).Delete(task.ID, &metav1.DeleteOptions{
      PropagationPolicy: &propagation,
    })
  }()
  // DEFER 获取 STDOUT
  // 默认只获取第一个 POD
  defer func() {
    selector := jobcreated.Spec.Selector.MatchLabels
    labelSelector := ""
    for key, value := range selector {
      if labelSelector != "" {
        labelSelector += "&&"
      }
      labelSelector += key + "=" + value
    }
    log.Println("labelSelector: ", labelSelector)
    pods, err := client.CoreV1().Pods(d.Namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
    if err != nil {
      log.Println("get pods error:", err)
      return
    }
    log.Printf("get %d pods\n", len(pods.Items))
    for i, podi := range pods.Items {
      log.Println("pod", i, podi.Name)
      req := client.CoreV1().Pods(d.Namespace).GetLogs(podi.Name, &corev1.PodLogOptions{
        //Follow:true,
      })
      body, err := req.DoRaw()
      if err != nil {
        log.Printf("k8s log %d err %#v", podi.Name, err)
      }
      fmt.Println("stdout size:", len(body))
      // todo: write body into stdout
      err = ioutil.WriteFile("job.log", body, 0755)
      if err != nil {
        log.Println("write job.log err", err)
      }
      return
      
      return
    }
  }()
  
  if d.Endpoints != nil && len(d.Endpoints) != 0 {
    ports := make([]corev1.ServicePort, len(d.Endpoints))
    service := &corev1.Service{
      TypeMeta: metav1.TypeMeta{
        Kind:       "Service",
        APIVersion: "v1",
      },
      ObjectMeta: metav1.ObjectMeta{
        Name:      task.ID,
        Namespace: d.Namespace,
      },
      Spec: corev1.ServiceSpec{
        Selector: podSelector,
        Ports:    ports,
      },
    }
    serviceCreated, err := client.CoreV1().Services(d.Namespace).Create(service)
    if err != nil {
      log.Println("create servcice err:", service, serviceCreated, err)
      return err
    }
    d.Meta("services", serviceCreated)
  }
  // to do -> convert it into for{}
  var logpodname string
  var doCopy = false
  
  f, err := os.OpenFile("follow.log", os.O_WRONLY|os.O_CREATE, 0666)
  if err != nil {
    log.Fatal("write job.log err", err)
  }
  defer f.Close()
  for i := 0; i < 10; i++ {
    jobcheck, err := client.BatchV1().Jobs(d.Namespace).Get(task.ID, metav1.GetOptions{})
    if err != nil {
      log.Println("err get job", err)
    }
    log.Printf("job %s status: %v %v %v %v %v %d", jobcheck.Name, jobcheck.Status, jobcheck.Status.String(),
      jobcheck.Status.Succeeded,
      jobcheck.Status.Failed,
      jobcheck.Status.Active,
      jobcheck.Status.Size())
    if jobcheck.Status.Active == 0 && jobcheck.Status.Succeeded+jobcheck.Status.Failed > 0 {
      i = 10
    }
    time.Sleep(time.Second * 3)
    
    // got log
    if logpodname == "" {
      
      selector := jobcreated.Spec.Selector.MatchLabels
      labelSelector := ""
      for key, value := range selector {
        if labelSelector != "" {
          labelSelector += "&&"
        }
        labelSelector += key + "=" + value
      }
      log.Println("labelSelector: ", labelSelector)
      pods, err := client.CoreV1().Pods(d.Namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
      if err != nil {
        log.Println("get pods error:", err)
        continue
      }
      if len(pods.Items) == 0 {
        continue
      }
      podi := pods.Items[0]
      logpodname = podi.Name
    } else if !doCopy {
      req := client.CoreV1().Pods(d.Namespace).GetLogs(logpodname, &corev1.PodLogOptions{
        Follow: true,
      })
      reader, err := req.Stream()
      if err != nil {
        continue
      }
      doCopy = true
      go func() {
        log.Println("start copy.")
        
        io.Copy(stdio.Stdout, reader)
        log.Println("io copy end.")
      }()
      
    }
  }
  return nil
}

func (d *K8sJob) Start(ctx context.Context, task *tug.StagedTask, stdio *tug.Stdio) (jobctl tug.RunningTaskController, err error) {
  
  // set appCommand
  var client *kubernetes.Clientset
  if c, got := d.Backend.Clusters[d.ClusterName]; got {
    client = c.client
  } else {
    err = errors.New("Bad Cluster Name")
    return
  }
  commands := task.Command
  var partition string = d.RuntimeParams.Partition()
  var jobname string = d.RuntimeParams.JobName()
  if jobname == "" {
    jobname = d.AppName
  }
  podLabelsSet := map[string]string{"app": d.AppName, "name": task.ID, "partition": partition, "jobname": jobname}
  podSelector := map[string]string{"name": task.ID}
  var ttlSecondsAfterFinished int32 = 3600 * 24
  var ttlSecondsAfterFinishedPt = &ttlSecondsAfterFinished
  var backoffLimit int32 = 0 // default is 6
  var replicas int32 = d.RuntimeParams.Node()
  /*
     经测试 ttlSecondsAfterFinished 机制在 POWER 集群上未生效，根据文档，应该是相关 K8S 特性未启用
     https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/
  */
  envs := []corev1.EnvVar{}
  requests := make(corev1.ResourceList)
  if d.RuntimeParams == nil {
    d.RuntimeParams = tug.RuntimeParams{}
  }
  requests[corev1.ResourceCPU] = d.RuntimeParams.CPU()
  //requests[corev1.ResourceGPU] = d.RuntimeParams.GPU()
  requests[corev1.ResourceMemory] = d.RuntimeParams.Memory()
  volumeMounts := make([]corev1.VolumeMount, 0)
  volumes := make([]corev1.Volume, 0)
  securityContext := &corev1.SecurityContext{
    Capabilities: &corev1.Capabilities{
      Add: []corev1.Capability{
        "SYS_ADMIN",
      },
    },
  }
  var podSecurityContext *corev1.PodSecurityContext
  if d.SetRunAs {
    podSecurityContext = &corev1.PodSecurityContext{
      RunAsUser:  &d.RunAsUid,
      RunAsGroup: &d.RunAsGid,
    }
  }
  // TO-DO
  // Copy inputs into remote fs.
  var workdirMounted bool
  for i, input := range task.Inputs {
    // host := filepath.Join( "/nfs_data" ,input.Path)
    host := input.Path
    container := task.Task.Inputs[i].Path
    volumeiName := "input" + fmt.Sprint(i)
    volumei := corev1.Volume{
      Name: volumeiName,
      VolumeSource: corev1.VolumeSource{
        HostPath: &corev1.HostPathVolumeSource{
          Path: host,
        },
      },
    }
    volumeMounti := corev1.VolumeMount{
      Name:      volumeiName,
      ReadOnly:  true,
      MountPath: container,
    }
    volumeMounts = append(volumeMounts, volumeMounti)
    volumes = append(volumes, volumei)
    if host == task.Workdir {
      workdirMounted = true
    }
  }
  
  // mount Workdir
  if task.Workdir != "" && !workdirMounted {
    host := task.Workdir
    container := task.Task.Workdir
    volumeiName := "workdir"
    volumei := corev1.Volume{
      Name: volumeiName,
      VolumeSource: corev1.VolumeSource{
        HostPath: &corev1.HostPathVolumeSource{
          Path: host,
        },
      },
    }
    volumeMounti := corev1.VolumeMount{
      Name:      volumeiName,
      MountPath: container,
    }
    volumeMounts = append(volumeMounts, volumeMounti)
    volumes = append(volumes, volumei)
  }
  var specTemplate = corev1.PodTemplateSpec{
    ObjectMeta: metav1.ObjectMeta{
      Labels: podLabelsSet,
    },
    
    Spec: corev1.PodSpec{
      //NodeSelector: map[string]string{ "name":"power20" },
      Containers: []corev1.Container{
        {
          Name:            task.ID,
          Image:           task.ContainerImage,
          ImagePullPolicy: "Always",
          //ImagePullPolicy: "Never",
          Command:    commands,
          Env:        envs,
          WorkingDir: task.Task.Workdir,
          Resources: corev1.ResourceRequirements{
            Requests: requests,
            Limits:   requests,
          },
          VolumeMounts:    volumeMounts,
          SecurityContext: securityContext,
        },
      }, // containers end
      Volumes:         volumes,
      SecurityContext: podSecurityContext,
      //RestartPolicy:   "Never", // supported values: "OnFailure", "Never"
    },
  }
  
  if d.Type == K8sJobTypeBatchJob {
    specTemplate.Spec.RestartPolicy = "Never"
    targetJob := &v1.Job{
      ObjectMeta: metav1.ObjectMeta{
        Name: task.ID,
      },
      Spec: v1.JobSpec{
        BackoffLimit:            &backoffLimit, // 重试次数
        TTLSecondsAfterFinished: ttlSecondsAfterFinishedPt,
        // kjob 的 select 由 k8s 根据 template 自动创建
        //Selector: &metav1.LabelSelector{MatchLabels: podSelector},
        Template: specTemplate,
      },
    }
    jobyaml, _ := yaml.Marshal(targetJob)
    log.Println("job.yaml:\n", string(jobyaml))
    var jobcreated *v1.Job
    jobcreated, err = client.BatchV1().Jobs(d.Namespace).Create(targetJob)
    if err != nil {
      log.Println("create job failed:", jobcreated, err)
      return
    }
    d.Meta("job", jobcreated)
    
  } else if d.Type == K8sJobTypeDeployment {
    targetDeploy := &appv1.Deployment{
      ObjectMeta: metav1.ObjectMeta{
        Name: task.ID,
      },
      Spec: appv1.DeploymentSpec{
        Replicas: &replicas,
        Selector: &metav1.LabelSelector{
          MatchLabels: podSelector,
        },
        Template: specTemplate,
      },
    }
    targetDeploy.Labels = podLabelsSet
    depyaml , _ := yaml.Marshal(targetDeploy)
    log.Println("deploy.yaml:\n", string(depyaml))
    var deploy *appv1.Deployment
    deploy, err = client.AppsV1().Deployments(d.Namespace).Create(targetDeploy)
    if err != nil {
      log.Println("create deployment failed:", deploy, err)
      return
    }
    d.Meta("deployment", deploy)
  }
  
  // 如果有需要，创建 endpoints
  if d.Endpoints != nil && len(d.Endpoints) != 0 {
    ports := make([]corev1.ServicePort, len(d.Endpoints))
    for i, epi := range d.Endpoints {
      ports[i] = corev1.ServicePort{
        Name: epi.Name,
        Port: int32( epi.Port) ,
        TargetPort: intstr.FromInt(epi.TargetPort),
      }
    }
    service := &corev1.Service{
      TypeMeta: metav1.TypeMeta{
        Kind:       "Service",
        APIVersion: "v1",
      },
      ObjectMeta: metav1.ObjectMeta{
        Name:      task.ID,
        Namespace: d.Namespace,
      },
      Spec: corev1.ServiceSpec{
        Selector: podSelector,
        Ports:    ports,
      },
    }
    var serviceCreated *corev1.Service
    serviceCreated, err = client.CoreV1().Services(d.Namespace).Create(service)
    if err != nil {
      log.Println("create servcice err:", service, serviceCreated, err)
      return
    }
    d.Meta("services", serviceCreated)
  }
  jobctl = &K8sJobCtl{Job: task.ID, Cluster: d.ClusterName, Namespace: d.Namespace}
  //return nil, nil
  return
}

//func (d *K8sJob) RecoverRunningTaskController(t *tug.StagedTask,index string) (tug.RunningTaskController , error) {
//  return nil, fmt.Errorf("Cannot recover local Docker controler")
//}

func (rl *K8sJobCtl) State() (s tug.TaskState, err error) {
  
  return tug.TaskRunning, nil
}

func (rl *K8sJobCtl) Kill(log tug.Logger, store tug.Storage) (err error) {
  //rl.cancel()
  // CHANNEL TIMEOUT
  rl.Wait(log, store)
  _, err = rl.State()
  return
}

func (rl *K8sJobCtl) Wait(log tug.Logger, store tug.Storage) (err error) {
  // 添加通用的资源回收逻辑
  //var me tug.MultiError
  //try := me.Try
  //defer func() {
  //  fmt.Println("\nDebug finish\n")
  //  err = me.Finish()
  //}()
  //defer func() {
  //  fmt.Println("\nDebug removeall\n")
  //  try(rl.Task.RemoveAll())
  //}()
  //defer func() {
  //  fmt.Println("\nDebug upload\n")
  //  try(tug.Upload(rl.ctx, rl.Task, store, log))
  //}()
  //defer func() {
  //  try(rl.stdio.Close())
  //}()
  
  //return rl.Cmd.Wait()
  return nil
}

func (rl *K8sJobCtl) Index() string {
  return fmt.Sprint(rl.Job)
}

func (d *K8sJob) RecoverRunningTaskController(t *tug.StagedTask, index string) (tug.RunningTaskController, error) {
  //return nil, fmt.Errorf("Cannot recover local runtime controler")
  return &K8sJobCtl{Cluster: d.ClusterName, Namespace: d.Namespace, Job: index}, nil
}
