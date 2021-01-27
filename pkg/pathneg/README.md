# Path Negotiation Package
This package contains the implementation of the path negotiation created by [Lukas Reichart](https://github.com/lukasreichart/)
for his Bachelor Thesis. This is a proof-of-concept implementation and there are several
areas where this implementation could and should be improved. Since they are listed and
discussed in the thesis, they are currently not listed here again.

## Files

| File | Description |
| ---- | ----------- |
| `message.go` | Contains the definitions of the messages used in the path negotiation protocol, as well as their serializations. |
| `message_transport.go` | Contains method to send protocol messages using the `appnet` package. |
| `path_evaluation.go` | Contains the code for the `PathEvluator` responsible for calculating preferences for the end-to-end paths used in the path negotiation protocol. |
| `path_neg.go` | Contains the implementation of the path negotiation protocol. |
| `path_selection.go` | Contains the implementation of the voting algorithms used to select paths in a negotiation based on the preferences of the partries involved in the negotiation. |
