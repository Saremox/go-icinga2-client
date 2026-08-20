# go-icinga2-client

Icinga2 API client.

## Getting started

```go
import "github.com/saremox/go-icinga2-client/icinga2"

icinga, err := icinga2.New(icinga2.WebClient{
		URL:               "https://icinga.somewhere.com:5665",
		Username:           "icinga",
		Password:           "secret",
		Debug:              true,
		DisableKeepAlives:  false})
```

### List hostgroups

```go
hostGroups, err := icinga.ListHostGroups("")
```

### Create a hostgroup

```go
icinga.CreateHostGroup(icinga2.HostGroup{Name: "mygroup"})
```

### Delete a hostgroup

```go
icinga.DeleteHostGroup("mygroup")
```

## Supported Icinga objects

So far, supported are hostgroups, hosts, services. Downtimes are supported
readonly.
