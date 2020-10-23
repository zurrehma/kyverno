package policyreport

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine/response"
)

//GeneratePRsFromEngineResponse generate Violations from engine responses
func GeneratePRsFromEngineResponse(ers []response.EngineResponse, log logr.Logger) (pvInfos []Info) {
	for _, er := range ers {
		// ignore creation of PV for resources that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			log.V(4).Info("resource does no have a name assigned yet, not creating a policy violation", "resource", er.PolicyResponse.Resource)
			continue
		}
		// skip when response succeed
		if os.Getenv("POLICY-TYPE") != common.PolicyReport {
			if er.IsSuccessful() {
				continue
			}
		}
		// build policy violation info
		pvInfos = append(pvInfos, buildPVInfo(er))
	}

	return pvInfos
}

// Builder builds Policy Violation struct
// this is base type of namespaced and cluster policy violation
type Builder interface {
	build(info Info)
}

type requestBuilder struct{}

func NewBuilder() *requestBuilder {
	return &requestBuilder{}
}

func (pvb *requestBuilder) Generate(info Info) kyverno.PolicyViolationTemplate {
	pv := pvb.build(info.PolicyName, info.Resource.GetKind(), info.Resource.GetNamespace(), info.Resource.GetName(), info.Rules)
	return *pv
}

func (pvb *requestBuilder) build(policy, kind, namespace, name string, rules []kyverno.ViolatedRule) *kyverno.PolicyViolationTemplate {

	pv := &kyverno.PolicyViolationTemplate{
		Spec: kyverno.PolicyViolationSpec{
			Policy: policy,
			ResourceSpec: kyverno.ResourceSpec{
				Kind:      kind,
				Name:      name,
				Namespace: namespace,
			},
			ViolatedRules: rules,
		},
	}
	labelMap := map[string]string{
		"policy":   pv.Spec.Policy,
		"resource": pv.Spec.ToKey(),
	}
	pv.SetLabels(labelMap)
	if namespace != "" {
		pv.SetNamespace(namespace)
	}
	pv.SetGenerateName(fmt.Sprintf("%s-", policy))
	return pv
}

func buildPVInfo(er response.EngineResponse) Info {
	info := Info{
		PolicyName: er.PolicyResponse.Policy,
		Resource:   er.PatchedResource,
		Rules:      buildViolatedRules(er),
	}
	return info
}

func buildViolatedRules(er response.EngineResponse) []kyverno.ViolatedRule {
	var violatedRules []kyverno.ViolatedRule
	for _, rule := range er.PolicyResponse.Rules {
		if os.Getenv("POLICY-TYPE") != common.PolicyReport {
			if rule.Success {
				continue
			}
		}
		vrule := kyverno.ViolatedRule{
			Name:    rule.Name,
			Type:    rule.Type,
			Message: rule.Message,
		}
		vrule.Check = report.StatusFail
		if rule.Success {
			vrule.Check = report.StatusPass
		}
		violatedRules = append(violatedRules, vrule)
	}
	return violatedRules
}
