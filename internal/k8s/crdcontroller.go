/*
 * Copyright 2019-2020 VMware, Inc.
 * All Rights Reserved.
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*   http://www.apache.org/licenses/LICENSE-2.0
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*/

package k8s

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"time"

	istiov1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"

	istiocrd "istio.io/client-go/pkg/clientset/versioned"
	istioinformers "istio.io/client-go/pkg/informers/externalversions"

	avicache "github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/internal/cache"
	"github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/internal/lib"
	"github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/internal/objects"
	"github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/internal/status"
	akov1alpha1 "github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/pkg/apis/ako/v1alpha1"
	akocrd "github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/pkg/client/v1alpha1/clientset/versioned"
	akoinformers "github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/pkg/client/v1alpha1/informers/externalversions"

	"github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/pkg/utils"

	"k8s.io/client-go/tools/cache"
)

func NewCRDInformers(cs akocrd.Interface) {
	var akoInformerFactory akoinformers.SharedInformerFactory

	akoInformerFactory = akoinformers.NewSharedInformerFactoryWithOptions(cs, time.Second*30)
	hostRuleInformer := akoInformerFactory.Ako().V1alpha1().HostRules()
	httpRuleInformer := akoInformerFactory.Ako().V1alpha1().HTTPRules()
	aviSettingsInformer := akoInformerFactory.Ako().V1alpha1().AviInfraSettings()

	lib.AKOControlConfig().SetCRDInformers(&lib.AKOCrdInformers{
		HostRuleInformer:        hostRuleInformer,
		HTTPRuleInformer:        httpRuleInformer,
		AviInfraSettingInformer: aviSettingsInformer,
	})
}

func NewIstioCRDInformers(cs istiocrd.Interface) {
	var istioInformerFactory istioinformers.SharedInformerFactory

	istioInformerFactory = istioinformers.NewSharedInformerFactoryWithOptions(cs, time.Second*30)
	vsInformer := istioInformerFactory.Networking().V1alpha3().VirtualServices()
	drInformer := istioInformerFactory.Networking().V1alpha3().DestinationRules()
	gatewayInformer := istioInformerFactory.Networking().V1alpha3().Gateways()

	lib.AKOControlConfig().SetIstioCRDInformers(&lib.IstioCRDInformers{
		VirtualServiceInformer:  vsInformer,
		DestinationRuleInformer: drInformer,
		GatewayInformer:         gatewayInformer,
	})
}

func isHostRuleUpdated(oldHostRule, newHostRule *akov1alpha1.HostRule) bool {
	if oldHostRule.ResourceVersion == newHostRule.ResourceVersion {
		return false
	}

	oldSpecHash := utils.Hash(utils.Stringify(oldHostRule.Spec) + oldHostRule.Status.Status)
	newSpecHash := utils.Hash(utils.Stringify(newHostRule.Spec) + newHostRule.Status.Status)

	return oldSpecHash != newSpecHash
}

func isHTTPRuleUpdated(oldHTTPRule, newHTTPRule *akov1alpha1.HTTPRule) bool {
	if oldHTTPRule.ResourceVersion == newHTTPRule.ResourceVersion {
		return false
	}

	oldSpecHash := utils.Hash(utils.Stringify(oldHTTPRule.Spec) + oldHTTPRule.Status.Status)
	newSpecHash := utils.Hash(utils.Stringify(newHTTPRule.Spec) + newHTTPRule.Status.Status)

	return oldSpecHash != newSpecHash
}

func isAviInfraUpdated(oldAviInfra, newAviInfra *akov1alpha1.AviInfraSetting) bool {
	if oldAviInfra.ResourceVersion == newAviInfra.ResourceVersion {
		return false
	}

	oldSpecHash := utils.Hash(utils.Stringify(oldAviInfra.Spec) + oldAviInfra.Status.Status)
	newSpecHash := utils.Hash(utils.Stringify(newAviInfra.Spec) + newAviInfra.Status.Status)

	return oldSpecHash != newSpecHash
}

// SetupAKOCRDEventHandlers handles setting up of AKO CRD event handlers
// TODO: The CRD are getting re-enqueued for the same resourceVersion via fullsync as well as via these handlers.
// We can leverage the resourceVersion checks to optimize this code. However the CRDs would need a check on
// status for re-publish. The status does not change the resourceVersion and during fullsync we ignore a CRD
// if it's status is not updated.
func (c *AviController) SetupAKOCRDEventHandlers(numWorkers uint32) {
	utils.AviLog.Infof("Setting up AKO CRD Event handlers")
	informer := lib.AKOControlConfig().CRDInformers()

	if lib.AKOControlConfig().HostRuleEnabled() {
		hostRuleEventHandler := cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if c.DisableSync {
					return
				}
				hostrule := obj.(*akov1alpha1.HostRule)
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(hostrule))
				key := lib.HostRule + "/" + utils.ObjKey(hostrule)
				if err := validateHostRuleObj(key, hostrule); err != nil {
					utils.AviLog.Warnf("key: %s, msg: Error retrieved during validation of HostRule: %v", key, err)
				}
				utils.AviLog.Debugf("key: %s, msg: ADD", key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			},
			UpdateFunc: func(old, new interface{}) {
				if c.DisableSync {
					return
				}
				oldObj := old.(*akov1alpha1.HostRule)
				hostrule := new.(*akov1alpha1.HostRule)
				if isHostRuleUpdated(oldObj, hostrule) {
					namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(hostrule))
					key := lib.HostRule + "/" + utils.ObjKey(hostrule)
					if err := validateHostRuleObj(key, hostrule); err != nil {
						utils.AviLog.Warnf("key: %s, Error retrieved during validation of HostRule: %v", key, err)
					}
					utils.AviLog.Debugf("key: %s, msg: UPDATE", key)
					bkt := utils.Bkt(namespace, numWorkers)
					c.workqueue[bkt].AddRateLimited(key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				if c.DisableSync {
					return
				}
				hostrule, ok := obj.(*akov1alpha1.HostRule)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						utils.AviLog.Errorf("couldn't get object from tombstone %#v", obj)
						return
					}
					hostrule, ok = tombstone.Obj.(*akov1alpha1.HostRule)
					if !ok {
						utils.AviLog.Errorf("Tombstone contained object that is not an HostRule: %#v", obj)
						return
					}
				}
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(hostrule))
				key := lib.HostRule + "/" + utils.ObjKey(hostrule)
				utils.AviLog.Debugf("key: %s, msg: DELETE", key)
				objects.SharedResourceVerInstanceLister().Delete(key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			},
		}

		informer.HostRuleInformer.Informer().AddEventHandler(hostRuleEventHandler)
	}

	if lib.AKOControlConfig().HttpRuleEnabled() {
		httpRuleEventHandler := cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if c.DisableSync {
					return
				}
				httprule := obj.(*akov1alpha1.HTTPRule)
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(httprule))
				key := lib.HTTPRule + "/" + utils.ObjKey(httprule)
				if err := validateHTTPRuleObj(key, httprule); err != nil {
					utils.AviLog.Warnf("Error retrieved during validation of HTTPRule: %v", err)
				}
				utils.AviLog.Debugf("key: %s, msg: ADD", key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			},
			UpdateFunc: func(old, new interface{}) {
				if c.DisableSync {
					return
				}
				oldObj := old.(*akov1alpha1.HTTPRule)
				httprule := new.(*akov1alpha1.HTTPRule)
				// reflect.DeepEqual does not work on type []byte,
				// unable to capture edits in destinationCA
				if isHTTPRuleUpdated(oldObj, httprule) {
					namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(httprule))
					key := lib.HTTPRule + "/" + utils.ObjKey(httprule)
					if err := validateHTTPRuleObj(key, httprule); err != nil {
						utils.AviLog.Warnf("Error retrieved during validation of HTTPRule: %v", err)
					}
					utils.AviLog.Debugf("key: %s, msg: UPDATE", key)
					bkt := utils.Bkt(namespace, numWorkers)
					c.workqueue[bkt].AddRateLimited(key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				if c.DisableSync {
					return
				}
				httprule, ok := obj.(*akov1alpha1.HTTPRule)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						utils.AviLog.Errorf("couldn't get object from tombstone %#v", obj)
						return
					}
					httprule, ok = tombstone.Obj.(*akov1alpha1.HTTPRule)
					if !ok {
						utils.AviLog.Errorf("Tombstone contained object that is not an HTTPRule: %#v", obj)
						return
					}
				}
				key := lib.HTTPRule + "/" + utils.ObjKey(httprule)
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(httprule))
				utils.AviLog.Debugf("key: %s, msg: DELETE", key)
				// no need to validate for delete handler
				bkt := utils.Bkt(namespace, numWorkers)
				objects.SharedResourceVerInstanceLister().Delete(key)
				c.workqueue[bkt].AddRateLimited(key)
			},
		}

		informer.HTTPRuleInformer.Informer().AddEventHandler(httpRuleEventHandler)
	}

	if lib.AKOControlConfig().AviInfraSettingEnabled() {
		aviInfraEventHandler := cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if c.DisableSync {
					return
				}
				aviinfra := obj.(*akov1alpha1.AviInfraSetting)
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(aviinfra))
				key := lib.AviInfraSetting + "/" + utils.ObjKey(aviinfra)
				if err := validateAviInfraSetting(key, aviinfra); err != nil {
					utils.AviLog.Warnf("Error retrieved during validation of AviInfraSetting: %v", err)
				}
				utils.AviLog.Debugf("key: %s, msg: ADD", key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			},
			UpdateFunc: func(old, new interface{}) {
				if c.DisableSync {
					return
				}
				oldObj := old.(*akov1alpha1.AviInfraSetting)
				aviInfra := new.(*akov1alpha1.AviInfraSetting)
				if isAviInfraUpdated(oldObj, aviInfra) {
					namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(aviInfra))
					key := lib.AviInfraSetting + "/" + utils.ObjKey(aviInfra)
					if err := validateAviInfraSetting(key, aviInfra); err != nil {
						utils.AviLog.Warnf("Error retrieved during validation of AviInfraSetting: %v", err)
					}
					utils.AviLog.Debugf("key: %s, msg: UPDATE", key)
					bkt := utils.Bkt(namespace, numWorkers)
					c.workqueue[bkt].AddRateLimited(key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				if c.DisableSync {
					return
				}
				aviinfra, ok := obj.(*akov1alpha1.AviInfraSetting)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						utils.AviLog.Errorf("couldn't get object from tombstone %#v", obj)
						return
					}
					aviinfra, ok = tombstone.Obj.(*akov1alpha1.AviInfraSetting)
					if !ok {
						utils.AviLog.Errorf("Tombstone contained object that is not an AviInfraSetting: %#v", obj)
						return
					}
				}
				key := lib.AviInfraSetting + "/" + utils.ObjKey(aviinfra)
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(aviinfra))
				utils.AviLog.Debugf("key: %s, msg: DELETE", key)
				objects.SharedResourceVerInstanceLister().Delete(key)
				// no need to validate for delete handler
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			},
		}

		informer.AviInfraSettingInformer.Informer().AddEventHandler(aviInfraEventHandler)
	}
	return
}

func (c *AviController) AddCrdIndexer() {
	informer := lib.AKOControlConfig().CRDInformers()
	if lib.AKOControlConfig().AviInfraSettingEnabled() {
		informer.AviInfraSettingInformer.Informer().AddIndexers(
			cache.Indexers{
				lib.SeGroupAviSettingIndex: func(obj interface{}) ([]string, error) {
					infraSetting, ok := obj.(*akov1alpha1.AviInfraSetting)
					if !ok {
						return []string{}, nil
					}
					return []string{infraSetting.Spec.SeGroup.Name}, nil
				},
			},
		)
	}
}

// SetupIstioCRDEventHandlers handles setting up of Istio CRD event handlers
func (c *AviController) SetupIstioCRDEventHandlers(numWorkers uint32) {
	utils.AviLog.Infof("Setting up AKO Istio CRD Event handlers")
	informer := lib.AKOControlConfig().IstioCRDInformers()

	virtualServiceEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			vs := obj.(*istiov1alpha3.VirtualService)
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(vs))
			key := lib.IstioVirtualService + "/" + utils.ObjKey(vs)
			utils.AviLog.Debugf("key: %s, msg: ADD", key)
			ok, resVer := objects.SharedResourceVerInstanceLister().Get(key)
			if ok && resVer.(string) == vs.ResourceVersion {
				utils.AviLog.Debugf("key: %s, msg: Same resource version returning", key)
				return
			}
			bkt := utils.Bkt(namespace, numWorkers)
			c.workqueue[bkt].AddRateLimited(key)
		},
		UpdateFunc: func(old, new interface{}) {
			if c.DisableSync {
				return
			}
			oldObj := old.(*istiov1alpha3.VirtualService)
			vs := new.(*istiov1alpha3.VirtualService)
			if !reflect.DeepEqual(oldObj.Spec, vs.Spec) {
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(vs))
				key := lib.IstioVirtualService + "/" + utils.ObjKey(vs)
				utils.AviLog.Debugf("key: %s, msg: UPDATE", key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			vs, ok := obj.(*istiov1alpha3.VirtualService)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utils.AviLog.Errorf("couldn't get object from tombstone %#v", obj)
					return
				}
				vs, ok = tombstone.Obj.(*istiov1alpha3.VirtualService)
				if !ok {
					utils.AviLog.Errorf("Tombstone contained object that is not an vs: %#v", obj)
					return
				}
			}
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(vs))
			key := lib.IstioVirtualService + "/" + utils.ObjKey(vs)
			utils.AviLog.Debugf("key: %s, msg: DELETE", key)
			bkt := utils.Bkt(namespace, numWorkers)
			objects.SharedResourceVerInstanceLister().Delete(key)
			c.workqueue[bkt].AddRateLimited(key)
		},
	}

	informer.VirtualServiceInformer.Informer().AddEventHandler(virtualServiceEventHandler)

	drEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			dr := obj.(*istiov1alpha3.DestinationRule)
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(dr))
			key := lib.IstioDestinationRule + "/" + utils.ObjKey(dr)
			utils.AviLog.Debugf("key: %s, msg: ADD", key)
			bkt := utils.Bkt(namespace, numWorkers)
			ok, resVer := objects.SharedResourceVerInstanceLister().Get(key)
			if ok && resVer.(string) == dr.ResourceVersion {
				utils.AviLog.Debugf("key: %s, msg: Same resource version returning", key)
				return
			}
			c.workqueue[bkt].AddRateLimited(key)
		},
		UpdateFunc: func(old, new interface{}) {
			if c.DisableSync {
				return
			}
			oldObj := old.(*istiov1alpha3.DestinationRule)
			dr := new.(*istiov1alpha3.DestinationRule)
			if !reflect.DeepEqual(oldObj.Spec, dr.Spec) {
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(dr))
				key := lib.IstioDestinationRule + "/" + utils.ObjKey(dr)
				utils.AviLog.Debugf("key: %s, msg: UPDATE", key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			dr, ok := obj.(*istiov1alpha3.DestinationRule)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utils.AviLog.Errorf("couldn't get object from tombstone %#v", obj)
					return
				}
				dr, ok = tombstone.Obj.(*istiov1alpha3.DestinationRule)
				if !ok {
					utils.AviLog.Errorf("Tombstone contained object that is not an vs: %#v", obj)
					return
				}
			}
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(dr))
			key := lib.IstioDestinationRule + "/" + utils.ObjKey(dr)
			utils.AviLog.Debugf("key: %s, msg: DELETE", key)
			bkt := utils.Bkt(namespace, numWorkers)
			objects.SharedResourceVerInstanceLister().Delete(key)
			c.workqueue[bkt].AddRateLimited(key)
		},
	}

	informer.DestinationRuleInformer.Informer().AddEventHandler(drEventHandler)

	gatewayEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			vs := obj.(*istiov1alpha3.Gateway)
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(vs))
			key := lib.IstioGateway + "/" + utils.ObjKey(vs)
			utils.AviLog.Debugf("key: %s, msg: ADD", key)
			ok, resVer := objects.SharedResourceVerInstanceLister().Get(key)
			if ok && resVer.(string) == vs.ResourceVersion {
				utils.AviLog.Debugf("key: %s, msg: Same resource version returning", key)
				return
			}
			bkt := utils.Bkt(namespace, numWorkers)
			c.workqueue[bkt].AddRateLimited(key)
		},
		UpdateFunc: func(old, new interface{}) {
			if c.DisableSync {
				return
			}
			oldObj := old.(*istiov1alpha3.Gateway)
			vs := new.(*istiov1alpha3.Gateway)
			if !reflect.DeepEqual(oldObj.Spec, vs.Spec) {
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(vs))
				key := lib.IstioGateway + "/" + utils.ObjKey(vs)
				utils.AviLog.Debugf("key: %s, msg: UPDATE", key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			vs, ok := obj.(*istiov1alpha3.Gateway)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utils.AviLog.Errorf("couldn't get object from tombstone %#v", obj)
					return
				}
				vs, ok = tombstone.Obj.(*istiov1alpha3.Gateway)
				if !ok {
					utils.AviLog.Errorf("Tombstone contained object that is not an vs: %#v", obj)
					return
				}
			}
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(vs))
			key := lib.IstioGateway + "/" + utils.ObjKey(vs)
			utils.AviLog.Debugf("key: %s, msg: DELETE", key)
			bkt := utils.Bkt(namespace, numWorkers)
			objects.SharedResourceVerInstanceLister().Delete(key)
			c.workqueue[bkt].AddRateLimited(key)
		},
	}

	informer.GatewayInformer.Informer().AddEventHandler(gatewayEventHandler)

}

// SetupMultiClusterIngressEventHandlers handles setting up of MultiClusterIngress CRD event handlers
func (c *AviController) SetupMultiClusterIngressEventHandlers(numWorkers uint32) {
	utils.AviLog.Infof("Setting up MultiClusterIngress CRD Event handlers")

	multiClusterIngressEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			mci := obj.(*akov1alpha1.MultiClusterIngress)
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(mci))
			key := lib.MultiClusterIngress + "/" + utils.ObjKey(mci)
			if lib.IsNamespaceBlocked(namespace) || !utils.CheckIfNamespaceAccepted(namespace) {
				utils.AviLog.Debugf("key: %s, msg: Multi-cluster Ingress add event: Namespace: %s didn't qualify filter. Not adding multi-cluster ingress", key, namespace)
				return
			}
			if err := validateMultiClusterIngressObj(key, mci); err != nil {
				utils.AviLog.Warnf("key: %s, msg: Validation of MultiClusterIngress failed: %v", key, err)
				return
			}
			utils.AviLog.Debugf("key: %s, msg: ADD", key)
			bkt := utils.Bkt(namespace, numWorkers)
			c.workqueue[bkt].AddRateLimited(key)
		},
		UpdateFunc: func(old, new interface{}) {
			if c.DisableSync {
				return
			}
			oldObj := old.(*akov1alpha1.MultiClusterIngress)
			mci := new.(*akov1alpha1.MultiClusterIngress)
			if !reflect.DeepEqual(oldObj.Spec, mci.Spec) {
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(mci))
				key := lib.MultiClusterIngress + "/" + utils.ObjKey(mci)
				if lib.IsNamespaceBlocked(namespace) || !utils.CheckIfNamespaceAccepted(namespace) {
					utils.AviLog.Debugf("key: %s, msg: Multi-cluster Ingress update event: Namespace: %s didn't qualify filter. Not updating multi-cluster ingress", key, namespace)
					return
				}
				if err := validateMultiClusterIngressObj(key, mci); err != nil {
					utils.AviLog.Warnf("key: %s, msg: Validation of MultiClusterIngress failed: %v", key, err)
					return
				}
				utils.AviLog.Debugf("key: %s, msg: UPDATE", key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			mci, ok := obj.(*akov1alpha1.MultiClusterIngress)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utils.AviLog.Errorf("couldn't get object from tombstone %#v", obj)
					return
				}
				mci, ok = tombstone.Obj.(*akov1alpha1.MultiClusterIngress)
				if !ok {
					utils.AviLog.Errorf("Tombstone contained object that is not a MultiClusterIngress: %#v", obj)
					return
				}
			}
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(mci))
			key := lib.MultiClusterIngress + "/" + utils.ObjKey(mci)
			if lib.IsNamespaceBlocked(namespace) || !utils.CheckIfNamespaceAccepted(namespace) {
				utils.AviLog.Debugf("key: %s, msg: Multi-cluster Ingress delete event: Namespace: %s didn't qualify filter. Not deleting multi-cluster ingress", key, namespace)
				return
			}
			utils.AviLog.Debugf("key: %s, msg: DELETE", key)
			bkt := utils.Bkt(namespace, numWorkers)
			objects.SharedResourceVerInstanceLister().Delete(key)
			c.workqueue[bkt].AddRateLimited(key)
		},
	}
	c.informers.MultiClusterIngressInformer.Informer().AddEventHandler(multiClusterIngressEventHandler)
}

// SetupServiceImportEventHandlers handles setting up of ServiceImport CRD event handlers
func (c *AviController) SetupServiceImportEventHandlers(numWorkers uint32) {
	utils.AviLog.Infof("Setting up ServiceImport CRD Event handlers")

	serviceImportEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			si := obj.(*akov1alpha1.ServiceImport)
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(si))
			key := lib.ServiceImport + "/" + utils.ObjKey(si)
			if lib.IsNamespaceBlocked(namespace) || !utils.CheckIfNamespaceAccepted(namespace) {
				utils.AviLog.Debugf("key: %s, msg: Service Import add event: Namespace: %s didn't qualify filter. Not adding Service Import", key, namespace)
				return
			}
			if err := validateServiceImportObj(key, si); err != nil {
				utils.AviLog.Warnf("key: %s, msg: Validation of ServiceImport failed: %v", key, err)
				return
			}
			utils.AviLog.Debugf("key: %s, msg: ADD", key)
			bkt := utils.Bkt(namespace, numWorkers)
			c.workqueue[bkt].AddRateLimited(key)
		},
		UpdateFunc: func(old, new interface{}) {
			if c.DisableSync {
				return
			}
			oldObj := old.(*akov1alpha1.ServiceImport)
			si := new.(*akov1alpha1.ServiceImport)
			if !reflect.DeepEqual(oldObj.Spec, si.Spec) {
				namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(si))
				key := lib.ServiceImport + "/" + utils.ObjKey(si)
				if lib.IsNamespaceBlocked(namespace) || !utils.CheckIfNamespaceAccepted(namespace) {
					utils.AviLog.Debugf("key: %s, msg: Service Import update event: Namespace: %s didn't qualify filter. Not updating Service Import", key, namespace)
					return
				}
				if err := validateServiceImportObj(key, si); err != nil {
					utils.AviLog.Warnf("key: %s, msg: Validation of ServiceImport failed: %v", key, err)
					return
				}
				utils.AviLog.Debugf("key: %s, msg: UPDATE", key)
				bkt := utils.Bkt(namespace, numWorkers)
				c.workqueue[bkt].AddRateLimited(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if c.DisableSync {
				return
			}
			si, ok := obj.(*akov1alpha1.ServiceImport)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utils.AviLog.Errorf("couldn't get object from tombstone %#v", obj)
					return
				}
				si, ok = tombstone.Obj.(*akov1alpha1.ServiceImport)
				if !ok {
					utils.AviLog.Errorf("Tombstone contained object that is not a ServiceImport: %#v", obj)
					return
				}
			}
			namespace, _, _ := cache.SplitMetaNamespaceKey(utils.ObjKey(si))
			key := lib.ServiceImport + "/" + utils.ObjKey(si)
			if lib.IsNamespaceBlocked(namespace) || !utils.CheckIfNamespaceAccepted(namespace) {
				utils.AviLog.Debugf("key: %s, msg: Service Import delete event: Namespace: %s didn't qualify filter. Not deleting Service Import", key, namespace)
				return
			}
			utils.AviLog.Debugf("key: %s, msg: DELETE", key)
			bkt := utils.Bkt(namespace, numWorkers)
			objects.SharedResourceVerInstanceLister().Delete(key)
			c.workqueue[bkt].AddRateLimited(key)
		},
	}
	c.informers.ServiceImportInformer.Informer().AddEventHandler(serviceImportEventHandler)
}

// validateHostRuleObj would do validation checks
// update internal CRD caches, and push relevant ingresses to ingestion
func validateHostRuleObj(key string, hostrule *akov1alpha1.HostRule) error {
	var err error
	fqdn := hostrule.Spec.VirtualHost.Fqdn
	foundHost, foundHR := objects.SharedCRDLister().GetFQDNToHostruleMapping(fqdn)
	if foundHost && foundHR != hostrule.Namespace+"/"+hostrule.Name {
		err = fmt.Errorf("duplicate fqdn %s found in %s", fqdn, foundHR)
		status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
		return err
	}

	// If it is not a Shared VS but TCP Settings are provided, then we reject it since these
	// TCP settings are not valid for the child VS.
	// TODO: move to translator?
	// if !strings.Contains(fqdn, lib.ShardVSSubstring) && hostrule.Spec.VirtualHost.TCPSettings != nil {
	// 	err = fmt.Errorf("Hostrule tcpSettings with fqdn %s cannot be applied to child Virtualservices", fqdn)
	// 	status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
	// 	return err
	// }

	if hostrule.Spec.VirtualHost.TCPSettings != nil && hostrule.Spec.VirtualHost.TCPSettings.LoadBalancerIP != "" {
		re := regexp.MustCompile(lib.IPRegex)
		if !re.MatchString(hostrule.Spec.VirtualHost.TCPSettings.LoadBalancerIP) {
			err = fmt.Errorf("loadBalancerIP %s is not a valid IP", hostrule.Spec.VirtualHost.TCPSettings.LoadBalancerIP)
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}
	}

	if hostrule.Spec.VirtualHost.Gslb.Fqdn != "" {
		if fqdn == hostrule.Spec.VirtualHost.Gslb.Fqdn {
			err = fmt.Errorf("GSLB FQDN and local FQDN are same")
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}
	}

	if hostrule.Spec.VirtualHost.TCPSettings != nil && len(hostrule.Spec.VirtualHost.TCPSettings.Listeners) > 0 {
		sslEnabled := false
		for _, listener := range hostrule.Spec.VirtualHost.TCPSettings.Listeners {
			if listener.EnableSSL {
				sslEnabled = true
				break
			}
		}
		if !sslEnabled {
			err = fmt.Errorf("Hosting parent virtualservice must have SSL enabled")
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}
	}

	if hostrule.Spec.VirtualHost.Aliases != nil {
		if hostrule.Spec.VirtualHost.FqdnType != akov1alpha1.Exact {
			err = fmt.Errorf("Aliases is supported only when FQDN type is set as Exact")
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}

		if utils.HasElem(hostrule.Spec.VirtualHost.Aliases, fqdn) {
			err = fmt.Errorf("Duplicate entry found. Aliases field has same entry as the FQDN field")
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}

		if utils.ContainsDuplicate(hostrule.Spec.VirtualHost.Aliases) {
			err = fmt.Errorf("Aliases must be unique")
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}

		if hostrule.Spec.VirtualHost.Gslb.Fqdn != "" &&
			utils.HasElem(hostrule.Spec.VirtualHost.Aliases, hostrule.Spec.VirtualHost.Gslb.Fqdn) {
			err = fmt.Errorf("Aliases must not contain GSLB FQDN")
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}

		for cachedFQDN, cachedAliases := range objects.SharedCRDLister().GetAllFQDNToAliasesMapping() {
			if cachedFQDN == fqdn {
				continue
			}
			aliases := cachedAliases.([]string)
			for _, alias := range hostrule.Spec.VirtualHost.Aliases {
				if utils.HasElem(aliases, alias) {
					err = fmt.Errorf("%s is already in use by hostrule %s", alias, cachedFQDN)
					status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
					return err
				}
			}
		}
	}

	refData := map[string]string{
		hostrule.Spec.VirtualHost.WAFPolicy:          "WafPolicy",
		hostrule.Spec.VirtualHost.ApplicationProfile: "AppProfile",
		hostrule.Spec.VirtualHost.TLS.SSLProfile:     "SslProfile",
		hostrule.Spec.VirtualHost.AnalyticsProfile:   "AnalyticsProfile",
		hostrule.Spec.VirtualHost.ErrorPageProfile:   "ErrorPageProfile",
	}
	if hostrule.Spec.VirtualHost.TLS.SSLKeyCertificate.Type == akov1alpha1.HostRuleSecretTypeAviReference {
		refData[hostrule.Spec.VirtualHost.TLS.SSLKeyCertificate.Name] = "SslKeyCert"
	}

	if hostrule.Spec.VirtualHost.TLS.SSLKeyCertificate.Type == akov1alpha1.HostRuleSecretTypeSecretReference {
		_, err := utils.GetInformers().SecretInformer.Lister().Secrets(hostrule.Namespace).Get(hostrule.Spec.VirtualHost.TLS.SSLKeyCertificate.Name)
		if err != nil {
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}
	}
	if hostrule.Spec.VirtualHost.TLS.SSLKeyCertificate.AlternateCertificate.Type == akov1alpha1.HostRuleSecretTypeAviReference {
		refData[hostrule.Spec.VirtualHost.TLS.SSLKeyCertificate.AlternateCertificate.Name] = "SslKeyCert"
	}

	if hostrule.Spec.VirtualHost.TLS.SSLKeyCertificate.AlternateCertificate.Type == akov1alpha1.HostRuleSecretTypeSecretReference {
		_, err := utils.GetInformers().SecretInformer.Lister().Secrets(hostrule.Namespace).Get(hostrule.Spec.VirtualHost.TLS.SSLKeyCertificate.AlternateCertificate.Name)
		if err != nil {
			status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
			return err
		}
	}

	for _, policy := range hostrule.Spec.VirtualHost.HTTPPolicy.PolicySets {
		refData[policy] = "HttpPolicySet"
	}

	for _, script := range hostrule.Spec.VirtualHost.Datascripts {
		refData[script] = "VsDatascript"
	}

	if err := checkRefsOnController(key, refData); err != nil {
		status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusRejected, Error: err.Error()})
		return err
	}

	// No need to update status of hostrule object as accepted since it was accepted before.
	if hostrule.Status.Status == lib.StatusAccepted {
		return nil
	}

	status.UpdateHostRuleStatus(key, hostrule, status.UpdateCRDStatusOptions{Status: lib.StatusAccepted, Error: ""})
	return nil
}

// validateMultiClusterIngressObj validates the MCI CRD changes before pushing it to ingestion
func validateMultiClusterIngressObj(key string, multiClusterIngress *akov1alpha1.MultiClusterIngress) error {

	var err error
	statusToUpdate := &akov1alpha1.MultiClusterIngressStatus{}
	defer func() {
		if err == nil {
			statusToUpdate.Status.Accepted = true
			status.UpdateMultiClusterIngressStatus(key, multiClusterIngress, statusToUpdate)
			return
		}
		statusToUpdate.Status.Accepted = false
		statusToUpdate.Status.Reason = err.Error()
		status.UpdateMultiClusterIngressStatus(key, multiClusterIngress, statusToUpdate)
	}()

	// Currently, we support only NodePort ServiceType.
	if !lib.IsNodePortMode() {
		err = fmt.Errorf("ServiceType must be of type NodePort")
		return err
	}

	// Currently, we support EVH mode only.
	if !lib.IsEvhEnabled() {
		err = fmt.Errorf("AKO must be in EVH mode")
		return err
	}

	if len(multiClusterIngress.Spec.Config) == 0 {
		err = fmt.Errorf("config must not be empty")
		return err
	}

	return nil
}

// validateServiceImportObj validates the SI CRD changes before pushing it to ingestion
func validateServiceImportObj(key string, serviceImport *akov1alpha1.ServiceImport) error {

	// CHECK ME: AMKO creates this and validation required?
	// TODO: validations needs a status field

	return nil
}

func checkRefsOnController(key string, refMap map[string]string) error {
	for k, value := range refMap {
		if k == "" {
			continue
		}

		if err := checkRefOnController(key, value, k); err != nil {
			return err
		}
	}
	return nil
}

var refModelMap = map[string]string{
	"SslKeyCert":             "sslkeyandcertificate",
	"WafPolicy":              "wafpolicy",
	"HttpPolicySet":          "httppolicyset",
	"SslProfile":             "sslprofile",
	"AppProfile":             "applicationprofile",
	"AnalyticsProfile":       "analyticsprofile",
	"ErrorPageProfile":       "errorpageprofile",
	"VsDatascript":           "vsdatascriptset",
	"HealthMonitor":          "healthmonitor",
	"ApplicationPersistence": "applicationpersistenceprofile",
	"PKIProfile":             "pkiprofile",
	"ServiceEngineGroup":     "serviceenginegroup",
	"Network":                "network",
}

// checkRefOnController checks whether a provided ref on the controller
func checkRefOnController(key, refKey, refValue string) error {
	// assign the last avi client for ref checks
	aviClientLen := lib.GetshardSize()
	clients := avicache.SharedAVIClients()
	uri := fmt.Sprintf("/api/%s?name=%s&fields=name,type,labels,created_by", refModelMap[refKey], refValue)

	// For public clouds, check using network UUID in AWS, normal network API for GCP, skip altogether for Azure.
	if lib.IsPublicCloud() && refModelMap[refKey] == "network" {
		if lib.UsesNetworkRef() {
			var rest_response interface{}
			utils.AviLog.Infof("Cloud is  %s, checking network ref using uuid", lib.GetCloudType())
			uri := fmt.Sprintf("/api/%s/%s?cloud_uuid=%s", refModelMap[refKey], refValue, lib.GetCloudUUID())
			err := lib.AviGet(clients.AviClient[aviClientLen], uri, &rest_response)
			if err != nil {
				utils.AviLog.Warnf("key: %s, msg: Get uri %v returned err %v", key, uri, err)
				return fmt.Errorf("%s \"%s\" not found on controller", refModelMap[refKey], refValue)
			} else if rest_response != nil {
				utils.AviLog.Infof("Found %s %s on controller", refModelMap[refKey], refValue)
				return nil
			} else {
				utils.AviLog.Warnf("key: %s, msg: No Objects found for refName: %s/%s", key, refModelMap[refKey], refValue)
				return fmt.Errorf("%s \"%s\" not found on controller", refModelMap[refKey], refValue)
			}
		}
	}

	result, err := lib.AviGetCollectionRaw(clients.AviClient[aviClientLen], uri)
	if err != nil {
		utils.AviLog.Warnf("key: %s, msg: Get uri %v returned err %v", key, uri, err)
		return fmt.Errorf("%s \"%s\" not found on controller", refModelMap[refKey], refValue)
	}

	if result.Count == 0 {
		utils.AviLog.Warnf("key: %s, msg: No Objects found for refName: %s/%s", key, refModelMap[refKey], refValue)
		return fmt.Errorf("%s \"%s\" not found on controller", refModelMap[refKey], refValue)
	}

	items := make([]json.RawMessage, result.Count)
	err = json.Unmarshal(result.Results, &items)
	if err != nil {
		utils.AviLog.Warnf("key: %s, msg: Failed to unmarshal results, err: %v", key, err)
		return fmt.Errorf("%s \"%s\" not found on controller", refModelMap[refKey], refValue)
	}

	item := make(map[string]interface{})
	err = json.Unmarshal(items[0], &item)
	if err != nil {
		utils.AviLog.Warnf("key: %s, msg: Failed to unmarshal item, err: %v", key, err)
		return fmt.Errorf("%s \"%s\" found on controller is invalid", refModelMap[refKey], refValue)
	}

	switch refKey {
	case "AppProfile":
		if appProfType, ok := item["type"].(string); ok && appProfType != lib.AllowedApplicationProfile {
			utils.AviLog.Warnf("key: %s, msg: applicationProfile: %s must be of type %s", key, refValue, lib.AllowedApplicationProfile)
			return fmt.Errorf("%s \"%s\" found on controller is invalid, must be of type: %s",
				refModelMap[refKey], refValue, lib.AllowedApplicationProfile)
		}
	case "ServiceEngineGroup":
		if seGroupLabels, ok := item["labels"].([]map[string]string); ok {
			if len(seGroupLabels) == 0 {
				utils.AviLog.Infof("key: %s, msg: ServiceEngineGroup %s not configured with labels", key, item["name"].(string))
			} else {
				if !reflect.DeepEqual(seGroupLabels, lib.GetLabels()) {
					utils.AviLog.Warnf("key: %s, msg: serviceEngineGroup: %s mismatched labels %s", key, refValue, utils.Stringify(seGroupLabels))
					return fmt.Errorf("%s \"%s\" found on controller is invalid, mismatched labels: %s",
						refModelMap[refKey], refValue, utils.Stringify(seGroupLabels))
				}
			}
		}
	}

	if itemCreatedBy, ok := item["created_by"].(string); ok && itemCreatedBy == lib.GetAKOUser() {
		utils.AviLog.Warnf("key: %s, msg: Cannot use object referred in CRD created by current AKO instance", key)
		return fmt.Errorf("%s \"%s\" Invalid operation, object referred is created by current AKO instance",
			refModelMap[refKey], refValue)
	}

	utils.AviLog.Infof("key: %s, msg: Ref found for %s/%s", key, refModelMap[refKey], refValue)
	return nil
}

// validateHTTPRuleObj would do validation checks
// update internal CRD caches, and push relevant ingresses to ingestion
func validateHTTPRuleObj(key string, httprule *akov1alpha1.HTTPRule) error {
	refData := make(map[string]string)
	for _, path := range httprule.Spec.Paths {
		if path.TLS.PKIProfile != "" && path.TLS.DestinationCA != "" {
			//if both pkiProfile and destCA set, reject httprule
			status.UpdateHTTPRuleStatus(key, httprule, status.UpdateCRDStatusOptions{
				Status: lib.StatusRejected,
				Error:  lib.HttpRulePkiAndDestCASetErr,
			})
			return fmt.Errorf("key: %s, msg: %s", key, lib.HttpRulePkiAndDestCASetErr)
		}
		refData[path.TLS.SSLProfile] = "SslProfile"
		refData[path.ApplicationPersistence] = "ApplicationPersistence"
		if path.TLS.PKIProfile != "" {
			refData[path.TLS.PKIProfile] = "PKIProfile"
		}

		for _, hm := range path.HealthMonitors {
			refData[hm] = "HealthMonitor"
		}
	}

	if err := checkRefsOnController(key, refData); err != nil {
		status.UpdateHTTPRuleStatus(key, httprule, status.UpdateCRDStatusOptions{
			Status: lib.StatusRejected,
			Error:  err.Error(),
		})
		return err
	}

	// No need to update status of httprule object as accepted since it was accepted before.
	if httprule.Status.Status == lib.StatusAccepted {
		return nil
	}

	status.UpdateHTTPRuleStatus(key, httprule, status.UpdateCRDStatusOptions{
		Status: lib.StatusAccepted,
		Error:  "",
	})
	return nil
}

// validateAviInfraSetting would do validaion checks on the
// ingested AviInfraSetting objects
func validateAviInfraSetting(key string, infraSetting *akov1alpha1.AviInfraSetting) error {
	if ((infraSetting.Spec.Network.EnableRhi != nil && !*infraSetting.Spec.Network.EnableRhi) || infraSetting.Spec.Network.EnableRhi == nil) &&
		len(infraSetting.Spec.Network.BgpPeerLabels) > 0 {
		err := fmt.Errorf("BGPPeerLabels cannot be set if EnableRhi is false.")
		status.UpdateAviInfraSettingStatus(key, infraSetting, status.UpdateCRDStatusOptions{
			Status: lib.StatusRejected,
			Error:  err.Error(),
		})
		return err
	}

	refData := make(map[string]string)
	for _, vipNetwork := range infraSetting.Spec.Network.VipNetworks {
		if vipNetwork.Cidr != "" {
			re := regexp.MustCompile(lib.IPCIDRRegex)
			if !re.MatchString(vipNetwork.Cidr) {
				err := fmt.Errorf("invalid CIDR configuration %s detected for networkName %s in vipNetworkList", vipNetwork.Cidr, vipNetwork.NetworkName)
				status.UpdateAviInfraSettingStatus(key, infraSetting, status.UpdateCRDStatusOptions{
					Status: lib.StatusRejected,
					Error:  err.Error(),
				})
				return err
			}
		}
		if vipNetwork.V6Cidr != "" {
			re := regexp.MustCompile(lib.IPV6CIDRRegex)
			if !re.MatchString(vipNetwork.V6Cidr) {
				err := fmt.Errorf("invalid IPv6 CIDR configuration %s detected for networkName %s in vipNetworkList", vipNetwork.V6Cidr, vipNetwork.NetworkName)
				status.UpdateAviInfraSettingStatus(key, infraSetting, status.UpdateCRDStatusOptions{
					Status: lib.StatusRejected,
					Error:  err.Error(),
				})
				return err
			}
		}
		refData[vipNetwork.NetworkName] = "Network"
	}

	if infraSetting.Spec.SeGroup.Name != "" {
		refData[infraSetting.Spec.SeGroup.Name] = "ServiceEngineGroup"
	}

	if err := checkRefsOnController(key, refData); err != nil {
		status.UpdateAviInfraSettingStatus(key, infraSetting, status.UpdateCRDStatusOptions{
			Status: lib.StatusRejected,
			Error:  err.Error(),
		})
		return err
	}

	// This would add SEG labels only if they are not configured yet. In case there is a label mismatch
	// to any pre-existing SEG labels, the AviInfraSettig CR will get Rejected from the checkRefsOnController
	// step before this.
	if infraSetting.Spec.SeGroup.Name != "" {
		addSeGroupLabel(key, infraSetting.Spec.SeGroup.Name)
	}

	// No need to update status of infra setting object as accepted since it was accepted before.
	if infraSetting.Status.Status == lib.StatusAccepted {
		return nil
	}

	status.UpdateAviInfraSettingStatus(key, infraSetting, status.UpdateCRDStatusOptions{
		Status: lib.StatusAccepted,
		Error:  "",
	})
	return nil
}

// addSeGroupLabel configures SEGroup with appropriate labels, during AviInfraSetting
// creation/updates after ingestion
func addSeGroupLabel(key, segName string) {
	// No need to configure labels if static route sync is disabled globally.
	if lib.GetDisableStaticRoute() {
		utils.AviLog.Infof("Skipping the check for SE group labels for SEG %s", segName)
		return
	}

	// assign the last avi client for ref checks
	clients := avicache.SharedAVIClients()
	aviClientLen := lib.GetshardSize()

	// configure labels on SeGroup if not present already.
	seGroup, err := avicache.GetAviSeGroup(clients.AviClient[aviClientLen], segName)
	if err != nil {
		utils.AviLog.Errorf("Failed to get SE group")
		return
	}

	avicache.ConfigureSeGroupLabels(clients.AviClient[aviClientLen], seGroup)
}
