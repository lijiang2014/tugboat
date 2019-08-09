# convert a local job into Cluster job

tugboat forked from [github.com/buchanae/tugboat](https://github.com/buchanae/tugboat), which only have a docker Executor and  not updated any more. 

* -> docker job ✅
* -> slurm job 
* -> k8s job
* -> k8s deployment ?

使作业在不同运行环境下运行；



例如， 本地运行 lammps 作业： 
 
 `lammps < sim.in` 

可以利用 cwl 来规范描述作业运行的细节

lammps.cwl:

```lammps.cwl
cwlVersion: v1.0
class: CommandLineTool
hints:
  - class: SoftwareRequirement
    packages:
    - package: lammps:
      version: $(inputs.version)
      specs: [ 'https://packages.debian.org/lammps' ]
  - class: DockerRequirement:
    dockerPull: lammps/lammps:stable_12Dec2018
inputs:
  - id: infile
    type: File
  - id: version
    type: string?
  - id: datafiles
    type:
      type: array
      items: File
outputs:
  output_file:
    type: stdout
  err_file:
    type: stderr
baseCommand: lammps
stdout: job.log
stderr: job.err
stdin: $(inputs.infile.path)
```

此次运行的参数如下

job.yml:

```yml
infile:
  class: File
  location: sim.in
```

这里描述了 运行时直接的命令（ baseCommand，  stdin ）,以及一些其他细节：
* DockerRequirement.dockerPull 对应包含了该软件的容器镜像
* inputs.datafiles 运行时隐含需要的其他输入文件
* 运行结束后需要保留 stdout, stderr
 
## LOCAL RUN
 
如果是本地运行，上诉的命令需要被解析为以下的执行命令

```bash
[/tmp/xxxxxx]$ lammps < sim.in 2> job.err > job.log
```

1. 创建 $TEMP/xxxxxx 临时文件夹（$TEMJOBDIR），Job 全部完成后释放；
    * 为了避免参数重名，后续的文件还会再建立二层的临时文件夹
2. 准备输入文件： 开始运行前，会将 inputs 中涉及的文件 在 此创建或创建链接
3. 执行命令，直到命令结束
4. 将 output 中涉及的文件 在 OUTDIR 中创建
5. 删除 $TEMJOBDIR 临时文件夹
 
# DOCKER RUN 
 
如果在docker 下运行，对应的执行命令则为

```bash
docker run -v $TEMJOBDIR:/cwl \
   -v  $inputs.FEILS[0].location:$TEMJOBDIR/$inputs.FEILS[0].path
   -v ... ... \
   -w $TEMJOBDIR:/cwl \
   -a stdin \
   lammps/lammps:stable_12Dec2018 \
   lammps \
   < $TEMJOBDIR/yyyyy/sim.in \
   2> $TEMJOBDIR/zzzzzz/job.err \
   > $TEMJOBDIR/zzzzzz/job.log \
```

## HPC ,以 SLURM SBATCH 为例

而如果在 SLURM 下运行

创建 job.sh

```
#!/bin/bash
   lammps
```

执行 
```bash
yhbatch -J xxxxx -i $TEMJOBDIR/yyyyy/sim.in  -e $TEMJOBDIR/zzzzzz/job.err  -o $TEMJOBDIR/zzzzzz/job.log  $TEMJOBDIR/rrrrr/job.sh
```

## KUBERNETES JOB

实际和使用 DOCKER 类似：

> kubectl 不方便操作 STDIN, STDOUT , 而实际的 JOB 的 STDIN 和 STDOUT 都是定位到文件，可以将其进行替换

创建 job-k8s-job.yaml : 

```
apiVersion: batch/v1
kind: Job
metadata:
  labels:
    app: lammps
  name: xxxxxx
spec:
  template:
    metadata:
      name: xxxxxx
    spec:
      containers:
        - name: xxxxxx-yyyyy
          image: lammps/lammps
          command:
          - sh 
          - '-c'
          - 'lammps < yyyyy/sim.in 2> zzzzzz/job.err > zzzzzz/job.log '
          workingDir: $TEMJOBDIR
          volumeMounts:
          - mountPath: /cwl
            name: cwl-vol        #必须有名称
          - mountPath: $Volume1  #被使用到的存储资源
            name: vol1
          - ...
      restartPolicy: Never
      volumes:
      - name: cwl-vol       #跟上面的名称对应
        hostPath: 
          path: $TEMJOBDIR      #宿主机挂载点
      - name: vol1
        hostPath: 
          path: $Volume1     #宿主机挂载点
      - ...
      ....
```



-----

# 补充说明 

* MODULE 

HPC 环境下需要加载环境变量，一般通过 module 进行管理；在我们自己发布的DOCKER镜像中也可以通过 module 来进行不同版本的软件的管理;
这里我们可以利用 CWL 的 SoftwareRequirement 来进行管理 ；SoftwareRequirement 有软件名 和 参数 字段。

我们可以将 SoftwareRequirement 解析为 module load 命令， 来 初始化 yhbatch 环境，或生成到 job.sh 脚本中。

如果 CWL 中的 SoftwareRequirement 可以有多条，那么可以进行环境的组合：
例如 假设此 lammps 是通过 动态库加载 fftw 环境，并有多个 FFTW 环境可以选，那么可以将 FFTW 环境的设置也创建一条 SoftwareRequirement 记录

* 针对镜像运行，SoftwareRequirement 除了可以解析为 module load $(software.name)/$(software.version) , 还可以有其他可能，比如CWL
 本身规范中建议的利用 spec 中的 IRI 来指导 软件的安装。

* MPI

CWL 更多是为 DOCKER 运行环境设计的，而 HPC 作业还有一个重要的运行时环境需要用户进行配置，即 MPI 环境。
标准的 CWL 中的字段没有直接进行相关的。

这里有两种方案：

* MPI 程序把 MPI 作为 baseCommand 进行设置
    * 比较简单，不需要额外工作，但是MPI 有各种不同的实现，不同实现的参数又有差异，会丧失通用性。
* 添加自定义的 MPIRequirement 

## MPIRequirement

```go
package cwl

type MPIRequirement struct {
  Mpirun string  
  Nodes int
  Cores int
  ...
}
``` 

存在 MPIRequirement 时， CMD 会根据 Mpirun 的 类似 对 CMD 进行再次包装，如针对 SLURM 环境：

```
#!/bin/bash
#SBATCH -N $(requirs.mpi.Nodes)
   srun -n $(requirs.mpi.cores) lammps
```

# Features

- [ ] localStorage + k


# Tests

- [x] lammps@local.docker
- [x] lammps@local
- [ ] lammps@slurm
- [ ] lammps@slurm.mpi
- [ ] lammps@local.docker.mpi
- [x] lammps@k8s.job
- [ ] tensorflow@local.docker
- [ ] tensorflow@local
- [ ] tensorflow@slurm
- [ ] tensorflow@slurm.mpi
- [ ] tensorflow@local.docker.mpi
- [ ] tensorflow@k8s.job
- [ ] tensorflow@k8s.deployment