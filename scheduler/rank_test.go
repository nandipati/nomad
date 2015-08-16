package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestFeasibleRankIterator(t *testing.T) {
	_, ctx := testContext(t)
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		nodes = append(nodes, mock.Node())
	}
	static := NewStaticIterator(ctx, nodes)

	feasible := NewFeasibleRankIterator(ctx, static)

	out := collectRanked(feasible)
	if len(out) != len(nodes) {
		t.Fatalf("bad: %v", out)
	}
}

func TestBinPackIterator_NoExistingAlloc(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
				Reserved: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Overloaded
				Resources: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
				Reserved: &structs.Resources{
					CPU:      512,
					MemoryMB: 512,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// 50% fit
				Resources: &structs.Resources{
					CPU:      4096,
					MemoryMB: 4096,
				},
				Reserved: &structs.Resources{
					CPU:      1024,
					MemoryMB: 1024,
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	resources := &structs.Resources{
		CPU:      1024,
		MemoryMB: 1024,
	}
	binp := NewBinPackIterator(ctx, static, resources, false, 0)

	out := collectRanked(binp)
	if len(out) != 2 {
		t.Fatalf("Bad: %v", out)
	}
	if out[0] != nodes[0] || out[1] != nodes[2] {
		t.Fatalf("Bad: %v", out)
	}

	if out[0].Score != 18 {
		t.Fatalf("Bad: %v", out[0])
	}
	if out[1].Score < 10 || out[1].Score > 16 {
		t.Fatalf("Bad: %v", out[1])
	}
}

func TestBinPackIterator_PlannedAlloc(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: mock.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: mock.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add a planned alloc to node1 that fills it
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			Resources: &structs.Resources{
				CPU:      2048,
				MemoryMB: 2048,
			},
		},
	}

	// Add a planned alloc to node2 that half fills it
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			Resources: &structs.Resources{
				CPU:      1024,
				MemoryMB: 1024,
			},
		},
	}

	resources := &structs.Resources{
		CPU:      1024,
		MemoryMB: 1024,
	}
	binp := NewBinPackIterator(ctx, static, resources, false, 0)

	out := collectRanked(binp)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}

	if out[0].Score != 18 {
		t.Fatalf("Bad: %v", out[0])
	}
}

func TestBinPackIterator_ExistingAlloc(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: mock.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: mock.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add existing allocations
	alloc1 := &structs.Allocation{
		ID:     mock.GenerateUUID(),
		EvalID: mock.GenerateUUID(),
		NodeID: nodes[0].Node.ID,
		JobID:  mock.GenerateUUID(),
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		Status: structs.AllocStatusPending,
	}
	alloc2 := &structs.Allocation{
		ID:     mock.GenerateUUID(),
		EvalID: mock.GenerateUUID(),
		NodeID: nodes[1].Node.ID,
		JobID:  mock.GenerateUUID(),
		Resources: &structs.Resources{
			CPU:      1024,
			MemoryMB: 1024,
		},
		Status: structs.AllocStatusPending,
	}
	noErr(t, state.UpdateAllocations(1000, nil, []*structs.Allocation{alloc1, alloc2}))

	resources := &structs.Resources{
		CPU:      1024,
		MemoryMB: 1024,
	}
	binp := NewBinPackIterator(ctx, static, resources, false, 0)

	out := collectRanked(binp)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[0].Score != 18 {
		t.Fatalf("Bad: %v", out[0])
	}
}

func TestBinPackIterator_ExistingAlloc_PlannedEvict(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: mock.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: mock.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add existing allocations
	alloc1 := &structs.Allocation{
		ID:     mock.GenerateUUID(),
		EvalID: mock.GenerateUUID(),
		NodeID: nodes[0].Node.ID,
		JobID:  mock.GenerateUUID(),
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		Status: structs.AllocStatusPending,
	}
	alloc2 := &structs.Allocation{
		ID:     mock.GenerateUUID(),
		EvalID: mock.GenerateUUID(),
		NodeID: nodes[1].Node.ID,
		JobID:  mock.GenerateUUID(),
		Resources: &structs.Resources{
			CPU:      1024,
			MemoryMB: 1024,
		},
		Status: structs.AllocStatusPending,
	}
	noErr(t, state.UpdateAllocations(1000, nil, []*structs.Allocation{alloc1, alloc2}))

	// Add a planned eviction to alloc1
	plan := ctx.Plan()
	plan.NodeEvict[nodes[0].Node.ID] = []string{alloc1.ID}

	resources := &structs.Resources{
		CPU:      1024,
		MemoryMB: 1024,
	}
	binp := NewBinPackIterator(ctx, static, resources, false, 0)

	out := collectRanked(binp)
	if len(out) != 2 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[0] || out[1] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[0].Score < 10 || out[0].Score > 16 {
		t.Fatalf("Bad: %v", out[0])
	}
	if out[1].Score != 18 {
		t.Fatalf("Bad: %v", out[0])
	}
}

func TestJobAntiAffinity_PlannedAlloc(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				ID: mock.GenerateUUID(),
			},
		},
		&RankedNode{
			Node: &structs.Node{
				ID: mock.GenerateUUID(),
			},
		},
	}
	static := NewStaticRankIterator(ctx, nodes)

	// Add a planned alloc to node1 that fills it
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			JobID: "foo",
		},
		&structs.Allocation{
			JobID: "foo",
		},
	}

	// Add a planned alloc to node2 that half fills it
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			JobID: "bar",
		},
	}

	binp := NewJobAntiAffinityIterator(ctx, static, 5.0, "foo")

	out := collectRanked(binp)
	if len(out) != 2 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0] != nodes[0] {
		t.Fatalf("Bad: %v", out)
	}
	if out[0].Score != -10.0 {
		t.Fatalf("Bad: %v", out[0])
	}

	if out[1] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}
	if out[1].Score != 0.0 {
		t.Fatalf("Bad: %v", out[1])
	}
}

func collectRanked(iter RankIterator) (out []*RankedNode) {
	for {
		next := iter.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}
	return
}
