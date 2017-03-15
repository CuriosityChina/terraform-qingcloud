package qingcloud

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	qc "github.com/yunify/qingcloud-sdk-go/service"
)

// func LoadbalancerTransitionStateRefresh(clt *loadbalancer.LOADBALANCER, id string) (interface{}, error) {
// 	refreshFunc := func() (interface{}, string, error) {
// 		params := loadbalancer.DescribeLoadBalancersRequest{}
// 		params.LoadbalancersN.Add(id)
// 		params.Verbose.Set(1)

// 		resp, err := clt.DescribeLoadBalancers(params)
// 		if err != nil {
// 			return nil, "", err
// 		}
// 		return resp.LoadbalancerSet[0], resp.LoadbalancerSet[0].TransitionStatus, nil
// 	}

// 	stateConf := &resource.StateChangeConf{
// 		Pending:    []string{"creating", "starting", "stopping", "updating", "suspending", "resuming", "deleting"},
// 		Target:     []string{""},
// 		Refresh:    refreshFunc,
// 		Timeout:    10 * time.Minute,
// 		Delay:      10 * time.Second,
// 		MinTimeout: 10 * time.Second,
// 	}
// 	return stateConf.WaitForState()
// }

// EipTransitionStateRefresh Waiting for no transition_status
func EIPTransitionStateRefresh(clt *qc.EIPService, id string) (interface{}, error) {
	refreshFunc := func() (interface{}, string, error) {
		input := new(qc.DescribeEIPsInput)
		input.EIPs = []*string{qc.String(id)}
		err := input.Validate()
		if err != nil {
			return nil, "", err
		}
		output, err := clt.DescribeEIPs(input)
		if err != nil {
			return nil, "", err
		}
		if output.RetCode != nil && qc.IntValue(output.RetCode) != 0 {
			return nil, "", fmt.Errorf("Error describe eip: %s", *output.Message)
		}
		if len(output.EIPSet) == 0 {
			return nil, "", fmt.Errorf("Error eip set is empty, request id %s", id)
		}
		return output.EIPSet[0], qc.StringValue(output.EIPSet[0].TransitionStatus), nil
	}
	stateConf := &resource.StateChangeConf{
		Pending:    []string{"associating", "dissociating", "suspending", "resuming", "releasing"},
		Target:     []string{""},
		Refresh:    refreshFunc,
		Timeout:    10 * time.Minute,
		Delay:      1 * time.Second,
		MinTimeout: 10 * time.Second,
	}
	return stateConf.WaitForState()
}

// func VolumeTransitionStateRefresh(clt *qc.VolumeService, id string) (interface{}, error) {
// 	refreshFunc := func() (interface{}, string, error) {
// 		params := volume.DescribeVolumesRequest{}
// 		params.VolumesN.Add(id)
// 		params.Verbose.Set(1)

// 		resp, err := clt.DescribeVolumes(params)
// 		if err != nil {
// 			return nil, "", err
// 		}
// 		return resp.VolumeSet[0], resp.VolumeSet[0].TransitionStatus, nil
// 	}

// 	stateConf := &resource.StateChangeConf{
// 		Pending:    []string{"creating", "attaching", "detaching", "suspending", "suspending", "resuming", "deleting", "recovering"}, // creating, attaching, detaching, suspending，resuming，deleting，recovering
// 		Target:     []string{""},
// 		Refresh:    refreshFunc,
// 		Timeout:    10 * time.Minute,
// 		Delay:      10 * time.Second,
// 		MinTimeout: 10 * time.Second,
// 	}
// 	return stateConf.WaitForState()
// }

// RouterTransitionStateRefresh Waiting for no transition_status
func RouterTransitionStateRefresh(clt *qc.RouterService, id string) (interface{}, error) {
	refreshFunc := func() (interface{}, string, error) {
		input := new(qc.DescribeRoutersInput)
		input.Routers = []*string{qc.String(id)}
		input.Verbose = qc.Int(1)
		err := input.Validate()
		if err != nil {
			return nil, "", fmt.Errorf("Error describe router validate input: %s", err)
		}
		output, err := clt.DescribeRouters(input)
		if err != nil {
			return nil, "", fmt.Errorf("Errorf describe router: %s", err)
		}
		if len(output.RouterSet) == 0 {
			return nil, "", fmt.Errorf("Error router set is empty, request id %s", id)
		}
		return output.RouterSet[0], qc.StringValue(output.RouterSet[0].TransitionStatus), nil
	}
	stateConf := &resource.StateChangeConf{
		Pending:    []string{"creating", "updating", "suspending", "resuming", "poweroffing", "poweroning", "deleting"},
		Target:     []string{""},
		Refresh:    refreshFunc,
		Timeout:    10 * time.Minute,
		Delay:      1 * time.Second,
		MinTimeout: 10 * time.Second,
	}
	return stateConf.WaitForState()
}

func InstanceTransitionStateRefresh(clt *qc.InstanceService, id string) (interface{}, error) {
	refreshFunc := func() (interface{}, string, error) {
		input := new(qc.DescribeInstancesInput)
		input.Instances = []*string{qc.String(id)}
		err := input.Validate()
		if err != nil {
			return nil, "", fmt.Errorf("Error describe instance input validate: %s", err)
		}
		output, err := clt.DescribeInstances(input)
		if err != nil {
			return nil, "", fmt.Errorf("Error describe instance: %s", err)
		}
		if output.RetCode != nil && qc.IntValue(output.RetCode) != 0 {
			return nil, "", fmt.Errorf("Error describe instance: %s", *output.Message)
		}
		if len(output.InstanceSet) == 0 {
			return nil, "", fmt.Errorf("Error eip set is empty, request id %s", id)
		}
		if qc.StringValue(output.InstanceSet[0].Status) == "terminated" || qc.StringValue(output.InstanceSet[0].Status) == "ceased" {
			return output.InstanceSet[0], "", nil
		}
		if qc.StringValue(output.InstanceSet[0].PrivateIP) == "" {
			return output.InstanceSet[0], "creating", nil
		}
		return output.InstanceSet[0], qc.StringValue(output.InstanceSet[0].TransitionStatus), nil
	}
	stateConf := &resource.StateChangeConf{
		Pending:    []string{"creating", "updating", "suspending", "resuming", "poweroffing", "poweroning", "deleting"},
		Target:     []string{""},
		Refresh:    refreshFunc,
		Timeout:    10 * time.Minute,
		Delay:      1 * time.Second,
		MinTimeout: 10 * time.Second,
	}
	return stateConf.WaitForState()
}

// func SecurityGroupTransitionStateRefresh(clt *qc.SecurityGroupService, id string) (interface{}, error) {
// 	refreshFunc := func() (interface{}, string, error) {
// 		input := new(qc.DescribeSecurityGroupsInput)
// 		input.SecurityGroups = []*string{qc.String(id)}
// 		err := input.Validate()
// 		if err != nil {
// 			return nil, "", fmt.Errorf("Error describe security group input validate: %s", err)
// 		}
// 		output, err := clt.DescribeSecurityGroups(input)
// 		if err != nil {
// 			return nil, "", fmt.Errorf("Error describe security group: %s", err)
// 		}
// 		if output.RetCode != nil && qc.IntValue(output.RetCode) != 0 {
// 			return nil, "", fmt.Errorf("Error describe security group: %s", err)
// 		}
// 		sg := output.SecurityGroupSet[0]
// 		if sg.IsApplied != nil && qc.IntValue(sg.IsApplied) == 1 {
// 			return nil, "", nil
// 		}
// 		return nil, "not_updated", nil
// 	}
// 	stateConf := &resource.StateChangeConf{
// 		Pending:    []string{"not_updated"},
// 		Target:     []string{""},
// 		Refresh:    refreshFunc,
// 		Timeout:    10 * time.Minute,
// 		Delay:      1 * time.Second,
// 		MinTimeout: 10 * time.Second,
// 	}
// 	return stateConf.WaitForState()
// }