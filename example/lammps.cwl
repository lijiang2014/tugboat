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
  - class: MpiRequirement:
    type: "slurm"
    mpiexec: "srun"
  - class: MpiRequirement:
    type: "openmpi"
    mpiexec: "/usr/lib64/openmpi/bin/mpiexec"
inputs:
  - id: infile
    type: File
    inputBinding: { position: 1, prefix: -in }
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
