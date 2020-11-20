# zapp
Tool to interact with Docker registry objects.  
This hooks in with registry auth stored by the docker CLI.  
This should work with any Docker or OCI compatible registry.

# Usage

## Inspect an image

```terminal
$ zapp docker.io/library/busybox:latest | jq .
{
  "manifests": [
    {
      "digest": "sha256:c9249fdf56138f0d929e2080ae98ee9cb2946f71498fc1484288e6a935b5e5bc",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      },
      "size": 527
    },
    {
      "digest": "sha256:a7c572c26ca470b3148d6c1e48ad3db90708a2769fdf836aa44d74b83190496d",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "arm",
        "os": "linux",
        "variant": "v5"
      },
      "size": 527
    },
    {
      "digest": "sha256:ce800872092c37c5f20ef111a5a69c5c8e94d0c5e055f76f530cb5e78a26ec03",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "arm",
        "os": "linux",
        "variant": "v6"
      },
      "size": 527
    },
    {
      "digest": "sha256:6655df04a3df853b029a5fac8836035ac4fab117800c9a6c4b69341bb5306c3d",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "arm",
        "os": "linux",
        "variant": "v7"
      },
      "size": 527
    },
    {
      "digest": "sha256:b8946184ce3ad6b4a09ebad2d85e81cfcaadc6897bfae2e9c6e2a4fe6afa6ee0",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "arm64",
        "os": "linux",
        "variant": "v8"
      },
      "size": 527
    },
    {
      "digest": "sha256:ba65e8d39e89b5c16f036c88c85952756777bf5385bce148bc44be48fac37d94",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "386",
        "os": "linux"
      },
      "size": 527
    },
    {
      "digest": "sha256:d7e83316d74e150866d82c45de342e78f662fe0aefbdb822d7d10c8b8e39cc4b",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "mips64le",
        "os": "linux"
      },
      "size": 527
    },
    {
      "digest": "sha256:0a11a95568b680dce6906a015bed88381e28ad17b31a63f7fec057b35573235a",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "ppc64le",
        "os": "linux"
      },
      "size": 528
    },
    {
      "digest": "sha256:426c855775f026d3fe76988b71938f4c9dc6840f09c0f29d8d4c75cc4238503b",
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "platform": {
        "architecture": "s390x",
        "os": "linux"
      },
      "size": 528
    }
  ],
  "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
  "schemaVersion": 2
}
```

### Drill down into that image

```terminal
$ zapp docker.io/library/busybox:latest@sha256:c9249fdf56138f0d929e2080ae98ee9cb2946f71498fc1484288e6a935b5e5bc
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "config": {
    "mediaType": "application/vnd.docker.container.image.v1+json",
    "size": 1493,
    "digest": "sha256:f0b02e9d092d905d0d87a8455a1ae3e9bb47b4aa3dc125125ca5cd10d6441c9f"
  },
  "layers": [
    {
      "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
      "size": 764619,
      "digest": "sha256:9758c28807f21c13d05c704821fdd56c0b9574912f9b916c65e1df3e6b8bc572"
    }
  ]
}
```

### Upload to a registry

```terminal
$ zapp docker.io/cpuguy83/somerepo ./path/to/content.tar.gz
```

Can upload any kind of content.
You can specify the media type after the content path (and as such skip mediatype detection).
