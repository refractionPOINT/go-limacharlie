package limacharlie

type LCClientBuilder interface {
	NewClient(opt ClientOptions) (LCClient, error)
}

type ClientBuilder struct {
	logger        LCLogger
	clientLoaders []ClientOptionLoader
}

var _ LCClientBuilder = &ClientBuilder{}

func NewClientBuilder(logger LCLogger, clientLoaders []ClientOptionLoader) *ClientBuilder {
	return &ClientBuilder{
		logger:        logger,
		clientLoaders: clientLoaders,
	}
}

func (b *ClientBuilder) NewClient(opt ClientOptions) (LCClient, error) {
	return NewClientFromLoader(opt, b.logger, b.clientLoaders...)
}
