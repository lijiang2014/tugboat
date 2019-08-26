package tugboat

import (
  "fmt"
  "k8s.io/apimachinery/pkg/api/resource"
  "strconv"
)

type RuntimeParams map[string]interface{}

func (p RuntimeParams) CPU() resource.Quantity {
  r , got := p["cpu"]
  if !got {
    r ="1"
  }
  rs := fmt.Sprint(r)
  return resource.MustParse(rs)
}


func (p RuntimeParams) GPU() resource.Quantity {
  r , got := p["gpu"]
  if !got {
    r ="0"
  }
  rs := fmt.Sprint(r)
  return resource.MustParse(rs)
}

func (p RuntimeParams) Memory() resource.Quantity {
  r , got := p["mem"]
  if !got {
    r , got = p["memory"]
  }
  if !got {
    r ="1"
  }
  rs := fmt.Sprint(r)
  if _, err := strconv.Atoi(rs); err == nil {
    rs += "Gi"
  }
  return resource.MustParse(rs)
}

func (p RuntimeParams) Node() int32 {
  r , got := p["node"]
  if !got {
    return 1
  }
  rs := fmt.Sprint(r)
  if v , err := strconv.Atoi(rs); err == nil {
    return int32(v)
  }
  return 1
}

func (p RuntimeParams) Partition() string {
  r , got := p["partition"]
  if !got {
    return "default"
  }
  rs := fmt.Sprint(r)
  return rs
}

func (p RuntimeParams) JobName() string {
  r , got := p["jobname"]
  if !got {
    return ""
  }
  rs := fmt.Sprint(r)
  return rs
}
