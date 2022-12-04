//go:build !(windows || linux || darwin)

package container

import docker "github.com/nektos/act/pkg/container/docker_stub"

var ImageExistsLocally = docker.ImageExistsLocally
var RemoveImage = docker.RemoveImage
var NewDockerBuildExecutor = docker.NewDockerBuildExecutor

type NewDockerBuildExecutorInput = docker.NewDockerBuildExecutorInput

var RunnerArch = docker.RunnerArch
var NewContainer = docker.NewContainer
var NewDockerVolumeRemoveExecutor = docker.NewDockerVolumeRemoveExecutor
var GetHostInfo = docker.GetHostInfo
