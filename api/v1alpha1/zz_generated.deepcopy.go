//go:build !ignore_autogenerated

// SPDX-License-Identifier: AGPL-3.0-only

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Authentication) DeepCopyInto(out *Authentication) {
	*out = *in
	if in.BearerToken != nil {
		in, out := &in.BearerToken, &out.BearerToken
		*out = new(BearerTokenAuthentication)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Authentication.
func (in *Authentication) DeepCopy() *Authentication {
	if in == nil {
		return nil
	}
	out := new(Authentication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Batch) DeepCopyInto(out *Batch) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Batch.
func (in *Batch) DeepCopy() *Batch {
	if in == nil {
		return nil
	}
	out := new(Batch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BearerTokenAuthentication) DeepCopyInto(out *BearerTokenAuthentication) {
	*out = *in
	out.SecretRef = in.SecretRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BearerTokenAuthentication.
func (in *BearerTokenAuthentication) DeepCopy() *BearerTokenAuthentication {
	if in == nil {
		return nil
	}
	out := new(BearerTokenAuthentication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExportPolicy) DeepCopyInto(out *ExportPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExportPolicy.
func (in *ExportPolicy) DeepCopy() *ExportPolicy {
	if in == nil {
		return nil
	}
	out := new(ExportPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ExportPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExportPolicyList) DeepCopyInto(out *ExportPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ExportPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExportPolicyList.
func (in *ExportPolicyList) DeepCopy() *ExportPolicyList {
	if in == nil {
		return nil
	}
	out := new(ExportPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ExportPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExportPolicySpec) DeepCopyInto(out *ExportPolicySpec) {
	*out = *in
	if in.Sources != nil {
		in, out := &in.Sources, &out.Sources
		*out = make([]TelemetrySource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Sink.DeepCopyInto(&out.Sink)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExportPolicySpec.
func (in *ExportPolicySpec) DeepCopy() *ExportPolicySpec {
	if in == nil {
		return nil
	}
	out := new(ExportPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExportPolicyStatus) DeepCopyInto(out *ExportPolicyStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExportPolicyStatus.
func (in *ExportPolicyStatus) DeepCopy() *ExportPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(ExportPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LocalSecretReference) DeepCopyInto(out *LocalSecretReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LocalSecretReference.
func (in *LocalSecretReference) DeepCopy() *LocalSecretReference {
	if in == nil {
		return nil
	}
	out := new(LocalSecretReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricSource) DeepCopyInto(out *MetricSource) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricSource.
func (in *MetricSource) DeepCopy() *MetricSource {
	if in == nil {
		return nil
	}
	out := new(MetricSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenTelemetryHTTP) DeepCopyInto(out *OpenTelemetryHTTP) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenTelemetryHTTP.
func (in *OpenTelemetryHTTP) DeepCopy() *OpenTelemetryHTTP {
	if in == nil {
		return nil
	}
	out := new(OpenTelemetryHTTP)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenTelemetrySink) DeepCopyInto(out *OpenTelemetrySink) {
	*out = *in
	in.Authentication.DeepCopyInto(&out.Authentication)
	if in.HTTP != nil {
		in, out := &in.HTTP, &out.HTTP
		*out = new(OpenTelemetryHTTP)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenTelemetrySink.
func (in *OpenTelemetrySink) DeepCopy() *OpenTelemetrySink {
	if in == nil {
		return nil
	}
	out := new(OpenTelemetrySink)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Retry) DeepCopyInto(out *Retry) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Retry.
func (in *Retry) DeepCopy() *Retry {
	if in == nil {
		return nil
	}
	out := new(Retry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SinkStatus) DeepCopyInto(out *SinkStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SinkStatus.
func (in *SinkStatus) DeepCopy() *SinkStatus {
	if in == nil {
		return nil
	}
	out := new(SinkStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TelemetrySink) DeepCopyInto(out *TelemetrySink) {
	*out = *in
	out.Batch = in.Batch
	out.Retry = in.Retry
	if in.OpenTelemetry != nil {
		in, out := &in.OpenTelemetry, &out.OpenTelemetry
		*out = new(OpenTelemetrySink)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TelemetrySink.
func (in *TelemetrySink) DeepCopy() *TelemetrySink {
	if in == nil {
		return nil
	}
	out := new(TelemetrySink)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TelemetrySource) DeepCopyInto(out *TelemetrySource) {
	*out = *in
	if in.Metrics != nil {
		in, out := &in.Metrics, &out.Metrics
		*out = new(MetricSource)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TelemetrySource.
func (in *TelemetrySource) DeepCopy() *TelemetrySource {
	if in == nil {
		return nil
	}
	out := new(TelemetrySource)
	in.DeepCopyInto(out)
	return out
}
