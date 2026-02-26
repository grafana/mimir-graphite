module github.com/grafana/mimir-graphite/v2

go 1.25.0

require (
	github.com/felixge/httpsnoop v1.0.4
	github.com/go-graphite/go-whisper v0.0.0-20230526115116-e3110f57c01c
	github.com/go-kit/log v0.2.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.4
	github.com/golang/snappy v1.0.0
	github.com/gorilla/mux v1.8.1
	github.com/grafana/dskit v0.0.0-20250422145853-90fa6b9a2b76
	github.com/grafana/metrictank v1.0.1-0.20230406204819-309ba74749c4
	github.com/grafana/mimir v0.0.0-20250501105506-4584085047c0
	github.com/kisielk/whisper-go v0.0.0-20140112135752-82e8091afdea
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f
	github.com/oklog/run v1.2.0
	github.com/oklog/ulid/v2 v2.1.1
	github.com/opentracing-contrib/go-stdlib v1.1.1
	github.com/opentracing/opentracing-go v1.2.1-0.20220228012449-10b1cf09e00b
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/common v0.67.5
	github.com/prometheus/prometheus v1.99.0
	github.com/stretchr/testify v1.11.1
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/uber/jaeger-lib v2.4.1+incompatible
	go.opentelemetry.io/otel v1.39.0
	go.opentelemetry.io/otel/bridge/opentracing v1.37.0
	go.opentelemetry.io/otel/sdk v1.39.0
	go.opentelemetry.io/otel/trace v1.39.0
	golang.org/x/net v0.51.0
	google.golang.org/grpc v1.79.1
	google.golang.org/protobuf v1.36.11
)

require (
	cloud.google.com/go v0.116.0 // indirect
	cloud.google.com/go/auth v0.16.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/storage v1.43.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.36.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.9 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.62 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.79.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.29.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.17 // indirect
	github.com/aws/smithy-go v1.22.3 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/hashicorp/go-msgpack/v2 v2.1.1 // indirect
	github.com/influxdata/influxdb/v2 v2.7.11 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/minio/crc64nvme v1.0.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/ncw/swift v1.0.53 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor v0.124.1 // indirect
	github.com/prometheus/sigv4 v0.1.2 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sercand/kuberesolver/v6 v6.0.0 // indirect
	github.com/tjhop/slog-gokit v0.1.4 // indirect
	github.com/twmb/franz-go/plugin/kotel v1.5.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector/component v1.30.0 // indirect
	go.opentelemetry.io/collector/confmap v1.30.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.124.0 // indirect
	go.opentelemetry.io/collector/consumer v1.30.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.124.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.124.0 // indirect
	go.opentelemetry.io/collector/processor v1.30.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.60.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	google.golang.org/genproto v0.0.0-20241113202542-65e8d215514f // indirect
)

require (
	cloud.google.com/go/iam v1.2.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.17.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.8.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.3.3 // indirect
	github.com/DmitriyVTitov/size v1.5.0 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go v1.55.6 // indirect
	github.com/bboreham/go-loser v0.0.0-20230920113527-fcc2c21820a3 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash v1.1.1-0.20190104011926-30d0e5bcb75d // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dgryski/go-jump v0.0.0-20211018200510-ba001c3ffce0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/efficientgo/core v1.0.0-rc.3 // indirect
	github.com/facette/natsort v0.0.0-20181210072756-2cd4dd1e2dcb // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.23.0 // indirect
	github.com/go-openapi/errors v0.22.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/loads v0.22.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.23.1 // indirect
	github.com/go-openapi/validate v0.24.0 // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/status v1.1.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/golang/glog v1.2.5 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grafana/gomemcache v0.0.0-20250318131618-74242eea118d // indirect
	github.com/grafana/regexp v0.0.0-20240607082908-2cb410fa05da // indirect
	github.com/hashicorp/consul/api v1.32.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.7 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/memberlist v0.5.1 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/miekg/dns v1.1.64 // indirect
	github.com/minio/minio-go/v7 v7.0.90 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opentracing-contrib/go-grpc v0.1.2 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/alertmanager v0.28.1 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	// indirect
	github.com/prometheus/otlptranslator v0.0.0-20250417063547-0a6a352a36dc // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/thanos-io/objstore v0.0.0-20250129163715-ec72e5a88a79 // indirect
	github.com/tinylib/msgp v1.1.8 // indirect
	github.com/twmb/franz-go v1.18.2-0.20250428225424-f2ead607417d // indirect
	github.com/twmb/franz-go/pkg/kadm v1.14.0 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.11.2 // indirect
	github.com/twmb/franz-go/plugin/kprom v1.1.0 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.5 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.5 // indirect
	go.etcd.io/etcd/client/v3 v3.5.5 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/featuregate v1.30.0 // indirect
	go.opentelemetry.io/collector/pdata v1.30.0 // indirect
	go.opentelemetry.io/collector/semconv v0.124.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	golang.org/x/tools v0.41.0 // indirect
	google.golang.org/api v0.229.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.32.3 // indirect
	k8s.io/client-go v0.32.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
)

replace (
	// Using a 3rd-party branch for custom dialer - see https://github.com/bradfitz/gomemcache/pull/86
	github.com/bradfitz/gomemcache => github.com/themihai/gomemcache v0.0.0-20180902122335-24332e2d58ab
	// Use fork of gocql that has gokit logs and Prometheus metrics.
	github.com/gocql/gocql => github.com/grafana/gocql v0.0.0-20200605141915-ba5dc39ece85

	// Copied from Mimir.
	github.com/prometheus/prometheus => github.com/grafana/mimir-prometheus v1.8.2-0.20250423083340-007de9b763aa

	// This replace comes from thanos, otherwise:
	// # github.com/sercand/kuberesolver
	// /Users/oleg/go/pkg/mod/github.com/sercand/kuberesolver@v2.1.0+incompatible/builder.go:108:82: undefined: resolver.BuildOption
	// /Users/oleg/go/pkg/mod/github.com/sercand/kuberesolver@v2.1.0+incompatible/builder.go:163:32: undefined: resolver.ResolveNowOption
	github.com/sercand/kuberesolver => github.com/sercand/kuberesolver v2.4.0+incompatible

	github.com/thanos-io/thanos v0.22.0 => github.com/thanos-io/thanos v0.19.1-0.20211125080947-19dcc7902d24
	// thanos requires a different version of galaxycache to build.
	github.com/vimeo/galaxycache => github.com/thanos-community/galaxycache v0.0.0-20211122094458-3a32041a1f1e

	// Copied from Mimir.
	gopkg.in/yaml.v3 => github.com/colega/go-yaml-yaml v0.0.0-20220720105220-255a8d16d094
)

exclude (
	// Exclude pre-go-mod kubernetes tags, as they are older
	// than v0.x releases but are picked when we update the dependencies.
	k8s.io/client-go v1.4.0
	k8s.io/client-go v1.4.0+incompatible
	k8s.io/client-go v1.5.0
	k8s.io/client-go v1.5.0+incompatible
	k8s.io/client-go v1.5.1
	k8s.io/client-go v1.5.1+incompatible
	k8s.io/client-go v2.0.0-alpha.1+incompatible
	k8s.io/client-go v2.0.0+incompatible
	k8s.io/client-go v3.0.0-beta.0+incompatible
	k8s.io/client-go v3.0.0+incompatible
	k8s.io/client-go v4.0.0-beta.0+incompatible
	k8s.io/client-go v4.0.0+incompatible
	k8s.io/client-go v5.0.0+incompatible
	k8s.io/client-go v5.0.1+incompatible
	k8s.io/client-go v6.0.0+incompatible
	k8s.io/client-go v7.0.0+incompatible
	k8s.io/client-go v8.0.0+incompatible
	k8s.io/client-go v9.0.0-invalid+incompatible
	k8s.io/client-go v9.0.0+incompatible
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/client-go v12.0.0+incompatible
)
