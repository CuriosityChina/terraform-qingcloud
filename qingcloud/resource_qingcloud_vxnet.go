package qingcloud

import (
	"errors"
	"fmt"
	// "log"

	// "github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	qc "github.com/yunify/qingcloud-sdk-go/service"
)

func resourceQingcloudVxnet() *schema.Resource {
	return &schema.Resource{
		Create: resourceQingcloudVxnetCreate,
		Read:   resourceQingcloudVxnetRead,
		Update: resourceQingcloudVxnetUpdate,
		Delete: resourceQingcloudVxnetDelete,
		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"type": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Description: "私有网络类型，1 - 受管私有网络，0 - 自管私有网络。	",
				ValidateFunc: withinArrayInt(0, 1),
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			// 当第一次创建一个私有网络以后，会首先加入到自己定制的router中，不是 vpc
			"router_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"vpc_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"ip_network": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateNetworkCIDR,
				Computed:     true,
			},
			"tag_ids": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"tag_names": &schema.Schema{
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
		},
	}
}

func resourceQingcloudVxnetCreate(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).vxnet

	vpcID := d.Get("vpc_id").(string)
	ipNetwork := d.Get("ip_network").(string)
	if (vpcID != "" && ipNetwork == "") || (vpcID == "" && ipNetwork != "") {
		return errors.New("router and ip_network must both be empty or no empty at the same time")
	}

	input := new(qc.CreateVxNetsInput)
	input.Count = qc.Int(1)
	input.VxNetName = qc.String(d.Get("name").(string))
	input.VxNetType = qc.Int(d.Get("type").(int))
	output, err := clt.CreateVxNets(input)
	if err != nil {
		return fmt.Errorf("Error create vxnet: %s", err)
	}
	if output.RetCode != nil && qc.IntValue(output.RetCode) != 0 {
		return fmt.Errorf("Error create vxnet: %s", *output.Message)
	}
	d.SetId(qc.StringValue(output.VxNets[0]))
	if err := modifyVxnetAttributes(d, meta, true); err != nil {
		return err
	}
	if vpcID != "" {
		if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, vpcID); err != nil {
			return err
		}
		qingcloudMutexKV.Lock(vpcID)
		defer qingcloudMutexKV.Unlock(vpcID)
		// join the router
		routerClt := meta.(*QingCloudClient).router
		joinRouterInput := new(qc.JoinRouterInput)
		joinRouterInput.VxNet = output.VxNets[0]
		joinRouterInput.Router = qc.String(vpcID)
		joinRouterInput.IPNetwork = qc.String(ipNetwork)

		joinRouterOutput, err := routerClt.JoinRouter(joinRouterInput)
		if err != nil {
			return fmt.Errorf("Error create vxnet join router: %s", err)
		}
		if joinRouterOutput.RetCode != nil && qc.IntValue(joinRouterOutput.RetCode) != 0 {
			return fmt.Errorf("Error create vxnet join router: %s", *joinRouterOutput.Message)
		}
		if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, vpcID); err != nil {
			return err
		}
	}
	if err := resourceUpdateTag(d, meta, qingcloudResourceTypeVxNet); err != nil {
		return err
	}
	return resourceQingcloudVxnetRead(d, meta)
}

func resourceQingcloudVxnetRead(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).vxnet
	input := new(qc.DescribeVxNetsInput)
	input.VxNets = []*string{qc.String(d.Id())}
	input.Verbose = qc.Int(1)
	output, err := clt.DescribeVxNets(input)
	if err != nil {
		return fmt.Errorf("Error describe vxnet: %s", err)
	}
	if output.RetCode == nil && qc.IntValue(output.RetCode) != 0 {
		return fmt.Errorf("Error describe vxnet: %s", *output.Message)
	}
	if len(output.VxNetSet) == 0 {
		return nil
	}
	vxnet := output.VxNetSet[0]
	d.Set("name", qc.StringValue(vxnet.VxNetName))
	d.Set("type", qc.IntValue(vxnet.VxNetType))
	d.Set("description", qc.StringValue(vxnet.Description))
	if vxnet.Router != nil {
		d.Set("router_id", qc.StringValue(vxnet.Router.RouterID))
		d.Set("ip_network", qc.StringValue(vxnet.Router.IPNetwork))
	} else {
		d.Set("router_id", "")
		d.Set("ip_network", "")
	}
	d.Set("vpc_id", qc.StringValue(vxnet.VpcRouterID))
	resourceSetTag(d, vxnet.Tags)
	return nil
}

func resourceQingcloudVxnetUpdate(d *schema.ResourceData, meta interface{}) error {
	if d.HasChange("vpc_id") || d.HasChange("ip_network") {
		routerClt := meta.(*QingCloudClient).router
		vpcID := d.Get("vpc_id").(string)
		IPNetwork := d.Get("ip_network").(string)
		if (vpcID != "" && IPNetwork == "") || (vpcID == "" && IPNetwork != "") {
			return errors.New("vpc_id and ip_network must both be empty or no empty at the same time")
		}
		oldVPC, newVPC := d.GetChange("vpc_id")
		oldVPCID := oldVPC.(string)
		newVPCID := newVPC.(string)
		oldV, _ := d.GetChange("router_id")
		oldRouterID := oldV.(string)
		if oldVPCID == "" {
			// do a join router action
			if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, newVPCID); err != nil {
				return err
			}
			joinRouterInput := new(qc.JoinRouterInput)
			joinRouterInput.VxNet = qc.String(d.Id())
			joinRouterInput.Router = qc.String(newVPCID)
			joinRouterInput.IPNetwork = qc.String(IPNetwork)
			joinRouterOutput, err := routerClt.JoinRouter(joinRouterInput)
			if err != nil {
				return fmt.Errorf("Error create vxnet join router: %s", err)
			}
			if joinRouterOutput.RetCode != nil && qc.IntValue(joinRouterOutput.RetCode) != 0 {
				return fmt.Errorf("Error create vxnet join router: %s", *joinRouterOutput.Message)
			}
			if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, newVPCID); err != nil {
				return err
			}
		} else if newVPCID == "" {
			// do a leave router action
			if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, oldVPCID); err != nil {
				return err
			}
			leaveRouterInput := new(qc.LeaveRouterInput)
			leaveRouterInput.Router = qc.String(oldRouterID)
			leaveRouterInput.VxNets = []*string{qc.String(d.Id())}
			leaveRouterOutput, err := routerClt.LeaveRouter(leaveRouterInput)
			if err != nil {
				return fmt.Errorf("Error leave router: %s", err)
			}
			if leaveRouterOutput.RetCode != nil && qc.IntValue(leaveRouterOutput.RetCode) != 0 {
				return fmt.Errorf("Error leave router: %s", *leaveRouterOutput.Message)
			}
			if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, oldVPCID); err != nil {
				return err
			}
		} else {
			// do a leave router then do a  join router action
			// leave router
			if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, oldVPCID); err != nil {
				return err
			}
			leaveRouterInput := new(qc.LeaveRouterInput)
			leaveRouterInput.Router = qc.String(oldRouterID)
			leaveRouterInput.VxNets = []*string{qc.String(d.Id())}
			leaveRouterOutput, err := routerClt.LeaveRouter(leaveRouterInput)
			if err != nil {
				return fmt.Errorf("Error leave router: %s", err)
			}
			if leaveRouterOutput.RetCode != nil && qc.IntValue(leaveRouterOutput.RetCode) != 0 {
				return fmt.Errorf("Error leave router: %s", *leaveRouterOutput.Message)
			}
			if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, oldVPCID); err != nil {
				return err
			}
			// join router
			if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, newVPCID); err != nil {
				return err
			}
			joinRouterInput := new(qc.JoinRouterInput)
			joinRouterInput.VxNet = qc.String(d.Id())
			joinRouterInput.Router = qc.String(newVPCID)
			joinRouterInput.IPNetwork = qc.String(IPNetwork)
			joinRouterOutput, err := routerClt.JoinRouter(joinRouterInput)
			if err != nil {
				return fmt.Errorf("Error create vxnet join router: %s", err)
			}
			if joinRouterOutput.RetCode != nil && qc.IntValue(joinRouterOutput.RetCode) != 0 {
				return fmt.Errorf("Error create vxnet join router: %s", *joinRouterOutput.Message)
			}
			if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, newVPCID); err != nil {
				return err
			}
		}
	}
	err := modifyVxnetAttributes(d, meta, false)
	if err != nil {
		return err
	}
	if err := resourceUpdateTag(d, meta, qingcloudResourceTypeVxNet); err != nil {
		return err
	}
	return resourceQingcloudVxnetRead(d, meta)
}

func resourceQingcloudVxnetDelete(d *schema.ResourceData, meta interface{}) error {
	clt := meta.(*QingCloudClient).vxnet

	if _, err := VxnetTransitionStateRefresh(clt, d.Id()); err != nil {
		return err
	}
	vpcID := d.Get("vpc_id").(string)
	routerID := d.Get("router_id").(string)
	// vxnet leave router
	if routerID != "" {
		if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, vpcID); err != nil {
			return err
		}
		routerCtl := meta.(*QingCloudClient).router
		leaveRouterInput := new(qc.LeaveRouterInput)
		leaveRouterInput.Router = qc.String(routerID)
		leaveRouterInput.VxNets = []*string{qc.String(d.Id())}
		leaveRouterOutput, err := routerCtl.LeaveRouter(leaveRouterInput)
		if err != nil {
			return fmt.Errorf("Error leave router: %s", err)
		}
		if leaveRouterOutput.RetCode != nil && qc.IntValue(leaveRouterOutput.RetCode) != 0 {
			return fmt.Errorf("Error leave router: %s", *leaveRouterOutput.Message)
		}
		if _, err := VxnetLeaveRouterTransitionStateRefresh(clt, d.Id()); err != nil {
			return err
		}
		if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, vpcID); err != nil {
			return err
		}
	}
	input := new(qc.DeleteVxNetsInput)
	input.VxNets = []*string{qc.String(d.Id())}
	output, err := clt.DeleteVxNets(input)
	if err != nil {
		return fmt.Errorf("Error delete vxnet: %s", err)
	}
	if output.RetCode != nil && qc.IntValue(output.RetCode) != 0 {
		return fmt.Errorf("Error delete vxnet: %s", *output.Message)
	}
	if _, err := RouterTransitionStateRefresh(meta.(*QingCloudClient).router, vpcID); err != nil {
		return err
	}
	d.SetId("")
	return nil
}
