Fork from https://github.com/tsenart/vegeta and add some function.

install:\
git clone https://git.100tal.com/jituan_zhiboyun_zby_qa/vegeta.git\
cd vegeta && go build -o vegeta

todo:\
1.目前在对每一个请求结果序列化输出时，性能损耗约10%，此处待优化<br>
2.目前发送http请求使用net/http，后期计划替换未fasthttp<br>
3.目前为反复生成请求对象和执行结果对象，会消耗内存区域开辟成本和GC成本，后续计划对此进行池化设计。