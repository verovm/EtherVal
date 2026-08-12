# EtherVal Implementation

This repository contains the code artifact for the following paper:

**Seongho Jeong, Yeonsoo Kim, Xiaowen Hu, Junhao Zhu, Bernd Burgstaller, Bernhard Scholz.**\
*EtherVal: Smart Contract Decompiler Validation for the Ethereum Blockchain.*
(Under review.)

EtherVal implementation extends the `substate-cli` command
of [verovm/record-replay](https://github.com/verovm/record-replay).
To build the EtherVal implementation,
first prepare prerequisites to build Geth v1.13 or record-replay rr0.5 on Linux
(specifically the Go 1.22 version from [here](https://go.dev/dl/)).
Then run `make` to build the `substate-cli` executable binary.
The subcommands added to `substate-cli` by EtherVal are under the `validate` category
from the `substate-cli --help` output.
```
   validate:
     record-trace                               replay EVM bytecodes and store EVM traces in DB
     validate-substrate, val-sbt                validate substrates with transactions and report output equivalence
     validate-substrate-tx, val-sbt-tx          validate substrate with one transaction and report output equivalence
     validate-tac, val-tac                      validate TAC with transactions and report output equivalence
     validate-tac-tx, val-tac-tx                validate TAC with one transaction and report output equivalence
     db-clone-trace                             Create a clone of substate DB with traces of a given block segment
     detect-deviation-substrate, dev-sbt        execute substrate with transactions until it deviates from bytecode
     detect-deviation-substrate-tx, dev-sbt-tx  execute substrate with a specific transaction until it deviates from bytecode
     detect-deviation-tac, dev-tac              execute TAC with transactions until it deviates from bytecode
     detect-deviation-tac-tx, dev-tac-tx        execute TAC with a specific transaction until it deviates from bytecode
     stat-tx-err                                Report statistics for OOG transactions
```

Visit [Artifacts & Data from EtherVal](https://elc.yonsei.ac.kr/etherval/) for data artifacts from EtherVal
including (1) EtherVal DBs with transaction states and Δ&#8209;semantics (EVM traces), (2) deployed EVM bytecode and decompiled TAC,
and (3) validation configurations and experimental results.
