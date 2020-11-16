package plugin

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/api/v1/pod"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

type BinPackingPlugin struct {
	handle framework.FrameworkHandle
}

const Name = "Bin-Packing-Plugin"

var _ framework.QueueSortPlugin = &BinPackingPlugin{}
var _ framework.ScorePlugin = &BinPackingPlugin{}
var _ framework.ScoreExtensions = &BinPackingPlugin{}

func (bppl *BinPackingPlugin) Name() string {
	return Name
}

func (bppl *BinPackingPlugin) Less(pInfo1, pInfo2 *framework.PodInfo) bool {
	/* 排序pod */
	p1 := pod.GetPodPriority(pInfo1.Pod)
	p2 := pod.GetPodPriority(pInfo2.Pod)
	if p1 != p2 {
		return p1 > p2
	}
	p1Cpu, p2Cpu, p1Mem, p2Mem := int64(0), int64(0), int64(0), int64(0)
	for _, container := range pInfo1.Pod.Spec.Containers {
		if cpuLimit, ok := container.Resources.Limits["cpu"]; ok {
			cpu, err := cpuLimit.AsInt64()
			if err != true {
				return pInfo1.Timestamp.Before(pInfo2.Timestamp)
			}
			p1Cpu += cpu
		}
		if memLimit, ok := container.Resources.Limits["memory"]; ok {
			mem, err := memLimit.AsInt64()
			if err != true {
				return pInfo1.Timestamp.Before(pInfo2.Timestamp)
			}
			p1Mem += mem
		}
	}
	for _, container := range pInfo2.Pod.Spec.Containers {
		if cpuLimit, ok := container.Resources.Limits["cpu"]; ok {
			cpu, err := cpuLimit.AsInt64()
			if err != true {
				return pInfo1.Timestamp.Before(pInfo2.Timestamp)
			}
			p2Cpu += cpu
		}
		if memLimit, ok := container.Resources.Limits["memory"]; ok {
			mem, err := memLimit.AsInt64()
			if err != true {
				return pInfo1.Timestamp.Before(pInfo2.Timestamp)
			}
			p2Mem += mem
		}
	}
	return p1Cpu*1024*1024+p1Mem > p2Cpu*1024*1024+p2Mem
}

func (bppl *BinPackingPlugin) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	/* 评分 优先调度pod多的node */
	nodeInfo, err := bppl.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}
	podNum := len(nodeInfo.Pods())
	return int64(podNum * 10), nil
}

func (bppl *BinPackingPlugin) NormalizeScore(ctx context.Context, state *framework.CycleState, p *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var (
		highest int64 = 0
		lowest        = scores[0].Score
	)
	for _, nodeScore := range scores {
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
	}

	if highest == lowest {
		lowest--
	}

	// Set Range to [0-100]
	for i, nodeScore := range scores {
		scores[i].Score = (nodeScore.Score - lowest) * framework.MaxNodeScore / (highest - lowest)
		klog.V(3).Infof("node: %v, final Score: %v", scores[i].Name, scores[i].Score)
	}
	return framework.NewStatus(framework.Success, "")
}

func (bppl *BinPackingPlugin) ScoreExtensions() framework.ScoreExtensions {
	return bppl
}

func New(configuration *runtime.Unknown, f framework.FrameworkHandle) (framework.Plugin, error) {
	return &BinPackingPlugin{
		handle: f,
	}, nil
}
