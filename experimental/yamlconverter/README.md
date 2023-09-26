# yamlconverter

## Purpose

This will take a Kubernetes YAML object and convert it into a native Go struct.
This implementation does have a few shortcomings and is not meant to be used as
a code generator. That said, it probably could be converted into one with a bit
more effort.

## Prior Art

As an example of prior art, have a look at
[NAML](https://github.com/krisnova/naml), which is how I discovered the
[valast](https://github.com/hexops/valast) library which this code makes
extensive use of. NAML assumes that one wants to immediately deploy their
generated objects, which is not always the case.

## Known Limitations

### runtime.RawExtensions
The runtime.RawExtensions types are not converted property into Go structs. See
the MachineConfig test case for further details.

### Custom Resource Definitions / Kube Object Types
As with NAML, this will not support all Kubernetes object types. Additional
object types can be added fairly easily however.
