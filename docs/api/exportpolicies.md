# API Reference

Packages:

- [telemetry.miloapis.com/v1alpha1](#telemetrydatumapiscomv1alpha1)

# telemetry.miloapis.com/v1alpha1

Resource Types:

- [ExportPolicy](#exportpolicy)




## ExportPolicy
<sup><sup>[↩ Parent](#telemetrydatumapiscomv1alpha1 )</sup></sup>






ExportPolicy is the Schema for the export policy API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>telemetry.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>ExportPolicy</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#exportpolicyspec">spec</a></b></td>
        <td>object</td>
        <td>
          Describes the expected state of the ExportPolicy's configuration. The
control plane will constantly evaluate the current state of exporters that
are deployed and ensure it matches the expected configuration. This field
is required when configuring an export policy.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#exportpolicystatus">status</a></b></td>
        <td>object</td>
        <td>
          Provides information on the current state of the export policy that was
observed by the control plane. This will be continuously updated as the
control plane monitors exporters.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.spec
<sup><sup>[↩ Parent](#exportpolicy)</sup></sup>



Describes the expected state of the ExportPolicy's configuration. The
control plane will constantly evaluate the current state of exporters that
are deployed and ensure it matches the expected configuration. This field
is required when configuring an export policy.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#exportpolicyspecsinksindex">sinks</a></b></td>
        <td>[]object</td>
        <td>
          Configures how telemetry data should be sent to a third-party telemetry
platforms.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#exportpolicyspecsourcesindex">sources</a></b></td>
        <td>[]object</td>
        <td>
          Defines how the export policy should source telemetry data to publish to
the configured sinks. An export policy can define multiple telemetry
sources. The export policy will **not** de-duplicate telemetry data that
matches multiple sources.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sinks[index]
<sup><sup>[↩ Parent](#exportpolicyspec)</sup></sup>



Configures how telemetry data should be sent to a third-party platform. As of
now there are no guarantees around delivery of telemetry data, especially if
the sink's endpoint is unavailable.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          A name provided to the telemetry sink that's unique within the export
policy.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>sources</b></td>
        <td>[]string</td>
        <td>
          A list of sources that should be sent to the telemetry sink.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#exportpolicyspecsinksindextarget">target</a></b></td>
        <td>object</td>
        <td>
          Configures the target of the telemetry sink.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sinks[index].target
<sup><sup>[↩ Parent](#exportpolicyspecsinksindex)</sup></sup>



Configures the target of the telemetry sink.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#exportpolicyspecsinksindextargetprometheusremotewrite">prometheusRemoteWrite</a></b></td>
        <td>object</td>
        <td>
          Configures the export policy to publish telemetry using the Prometheus
Remote Write protocol.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sinks[index].target.prometheusRemoteWrite
<sup><sup>[↩ Parent](#exportpolicyspecsinksindextarget)</sup></sup>



Configures the export policy to publish telemetry using the Prometheus
Remote Write protocol.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#exportpolicyspecsinksindextargetprometheusremotewritebatch">batch</a></b></td>
        <td>object</td>
        <td>
          Configures how telemetry data should be batched before sending to the sink.
By default, the sink will batch telemetry data every 5 seconds or when
the batch size reaches 500 entries, whichever comes first.<br/>
          <br/>
            <i>Default</i>: map[maxSize:500 timeout:5s]<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>endpoint</b></td>
        <td>string</td>
        <td>
          Configure an HTTP endpoint to use for publishing telemetry data.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#exportpolicyspecsinksindextargetprometheusremotewriteretry">retry</a></b></td>
        <td>object</td>
        <td>
          Configures the export policies' retry behavior when it fails to send
requests to the sink's endpoint. There's no guarantees that the export
policy will retry until success if the endpoint is not available or
configured incorrectly.<br/>
          <br/>
            <i>Default</i>: map[backoffDuration:5s maxAttempts:3]<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#exportpolicyspecsinksindextargetprometheusremotewriteauthentication">authentication</a></b></td>
        <td>object</td>
        <td>
          Configures how the sink should authenticate with the HTTP endpoint.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sinks[index].target.prometheusRemoteWrite.batch
<sup><sup>[↩ Parent](#exportpolicyspecsinksindextargetprometheusremotewrite)</sup></sup>



Configures how telemetry data should be batched before sending to the sink.
By default, the sink will batch telemetry data every 5 seconds or when
the batch size reaches 500 entries, whichever comes first.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>maxSize</b></td>
        <td>integer</td>
        <td>
          Maximum number of telemetry entries per batch.<br/>
          <br/>
            <i>Minimum</i>: 1<br/>
            <i>Maximum</i>: 5000<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>timeout</b></td>
        <td>string</td>
        <td>
          Batch timeout before sending telemetry. Must be a duration (e.g. 5s).<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sinks[index].target.prometheusRemoteWrite.retry
<sup><sup>[↩ Parent](#exportpolicyspecsinksindextargetprometheusremotewrite)</sup></sup>



Configures the export policies' retry behavior when it fails to send
requests to the sink's endpoint. There's no guarantees that the export
policy will retry until success if the endpoint is not available or
configured incorrectly.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>backoffDuration</b></td>
        <td>string</td>
        <td>
          Backoff duration that should be used to backoff when retrying requests.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>maxAttempts</b></td>
        <td>integer</td>
        <td>
          Maximum number of attempts before telemetry data should be dropped.<br/>
          <br/>
            <i>Minimum</i>: 1<br/>
            <i>Maximum</i>: 10<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sinks[index].target.prometheusRemoteWrite.authentication
<sup><sup>[↩ Parent](#exportpolicyspecsinksindextargetprometheusremotewrite)</sup></sup>



Configures how the sink should authenticate with the HTTP endpoint.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#exportpolicyspecsinksindextargetprometheusremotewriteauthenticationbasicauth">basicAuth</a></b></td>
        <td>object</td>
        <td>
          Configures the sink to use basic auth to authenticate with the configured
endpoint.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sinks[index].target.prometheusRemoteWrite.authentication.basicAuth
<sup><sup>[↩ Parent](#exportpolicyspecsinksindextargetprometheusremotewriteauthentication)</sup></sup>



Configures the sink to use basic auth to authenticate with the configured
endpoint.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#exportpolicyspecsinksindextargetprometheusremotewriteauthenticationbasicauthsecretref">secretRef</a></b></td>
        <td>object</td>
        <td>
          Configures which secret is used to retrieve the bearer token to add to the
authorization header. Secret must be a `kubernetes.io/basic-auth` type.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sinks[index].target.prometheusRemoteWrite.authentication.basicAuth.secretRef
<sup><sup>[↩ Parent](#exportpolicyspecsinksindextargetprometheusremotewriteauthenticationbasicauth)</sup></sup>



Configures which secret is used to retrieve the bearer token to add to the
authorization header. Secret must be a `kubernetes.io/basic-auth` type.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          The name of the secret<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sources[index]
<sup><sup>[↩ Parent](#exportpolicyspec)</sup></sup>



Defines how the export policy should source telemetry data from resources on
the platform.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          A unique name given to the telemetry source within an export policy. Must
be a valid DNS label.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#exportpolicyspecsourcesindexmetrics">metrics</a></b></td>
        <td>object</td>
        <td>
          Configures how the telemetry source should retrieve metric data from the
Datum Cloud platform.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.spec.sources[index].metrics
<sup><sup>[↩ Parent](#exportpolicyspecsourcesindex)</sup></sup>



Configures how the telemetry source should retrieve metric data from the
Datum Cloud platform.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>metricsql</b></td>
        <td>string</td>
        <td>
          The MetricSQL option allows to user to provide a metricsql query that can
be used to select and filter metric data that should be published by the
export policy.

Here's an example of a metricsql query that will publish gateway metrics:

``` {service_name=“networking.miloapis.com”, resource_kind="Gateway"} ```

See: https://docs.victoriametrics.com/metricsql/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.status
<sup><sup>[↩ Parent](#exportpolicy)</sup></sup>



Provides information on the current state of the export policy that was
observed by the control plane. This will be continuously updated as the
control plane monitors exporters.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#exportpolicystatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Provides summary status information on the export policy as a whole. Review
the sink status information for detailed information on each sink.

Known condition types are: "Ready"<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#exportpolicystatussinksindex">sinks</a></b></td>
        <td>[]object</td>
        <td>
          Provides status information on each sink that's configured.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.status.conditions[index]
<sup><sup>[↩ Parent](#exportpolicystatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.status.sinks[index]
<sup><sup>[↩ Parent](#exportpolicystatus)</sup></sup>



SinkStatus provides status information on the current status of a sink. This
can be used to determine whether a sink is configured correctly and is
exporting telemetry data.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          The name of the corresponding sink configuration in the spec of the export
policy.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#exportpolicystatussinksindexconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Provides status information on the current status of the sink. This can be
used to determine whether a sink is configured correctly and is exporting
telemetry data.

Known condition types are: "Ready"<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ExportPolicy.status.sinks[index].conditions[index]
<sup><sup>[↩ Parent](#exportpolicystatussinksindex)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>
