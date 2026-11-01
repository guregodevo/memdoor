# Run Memdoor on an open-weight model, no key

An open-weight model on your machine is a provider like any other. `memdoor connect local` finds it and asks for no
token. Nothing you type or read leaves the box. This page goes from an empty
machine to the team-solve workflow on your GPU.

## 1. Run a model

Any OpenAI-compatible server works. Four common ones:

| runtime | start it | listens on |
|---|---|---|
| [Ollama](https://ollama.com) | `ollama pull qwen3:8b` | 11434 |
| [llama.cpp](https://github.com/ggml-org/llama.cpp) | `llama-server -m model.gguf -c 32768` | 8080 |
| [LM Studio](https://lmstudio.ai) | load a model, start the server | 1234 |
| [vLLM](https://github.com/vllm-project/vllm) | `vllm serve Qwen/Qwen3.8-27B` | 8000 |

[janis](https://github.com/guregodevo/janis), our own server for Apple
silicon and CUDA, listens on 8090 and answers the decision model's
questions, so the token savings run locally too.

Which model: a coder turn reads files, calls tools and writes patches, so it
needs a model trained for tool calls. Qwen3.8-27B is the one the published
result ran on. A 7B or 8B answers a turn on a laptop but loses the thread on
long work. The floor for the team-solve workflow is a 27B at Q5 or above,
which needs a 24 GB card or a 32 GB Mac.

## 2. Give it a window

Ollama reads a prompt through 4,096 tokens unless told otherwise, whatever
the model allows. A coder turn's prompt is longer than that. The model
answers a task it never saw in a few lines and calls no tool. Start Ollama
with `OLLAMA_CONTEXT_LENGTH=32768` or make a tag with the window set:

```
printf 'FROM qwen3:8b\nPARAMETER num_ctx 32768\n' > Modelfile
ollama create qwen3:8b-32k -f Modelfile
```

llama.cpp takes `-c 32768` on the command line. LM Studio and vLLM set it in
the model's settings. 32K is enough for a turn. The team-solve workflow's
longest replies run past 100K, so give the 27B what the card allows.

## 3. Connect

```
curl -fsSL https://memdoor.ai/install.sh | bash
memdoor connect local            # finds the runtime on this machine, probes it, keeps it
memdoor model                    # the model that answers, and what it lists for: nothing
```

A blank address finds janis, Ollama, llama.cpp or LM Studio on their usual
ports. `--base http://localhost:8000/v1` names another. The probe reads the
runtime's model list and makes one call. A large model may take a few minutes
to load on the first one. `/model search` in the window lists the models the
runtime serves and `/model qwen3:8b-32k` pins one. A model pulled after the
connect is found on its next pin.

The runtime can be on a box instead of the laptop. `memdoor --gateway
ssh://you@box connect local` installs Memdoor there and connects the box's
runtime. `@box:coder …` in a conversation here runs the turn on the box
([peers](/docs/providers#the-box-s-model-answers-the-laptop)).

## 4. A turn

```
cd your-project && memdoor tui
```

The first turn tells you whether the model is up to the work. Ask it to read a
file and fix something small. The meter (`memdoor meter`) shows the input
tokens of each call. 4,096 on every call means the window of step 2 is not
set.

## 5. The team on your GPU

The [team-solve workflow](/docs/team-solve) names no model, so the
harness that scored 9.8 of 10 on LiveCodeBench Hard runs on whatever
`connect local` found:

```
git clone --depth 1 https://github.com/guregodevo/memdoor
mkdir -p ~/.memdoor/workflows && cp -r memdoor/examples/workflows/team-solve ~/.memdoor/workflows/
mkdir my-problem && cd my-problem && cp -r ../memdoor/examples/workflows/team-solve/sample/* .
memdoor workflow run team-solve --wait
```

A hard problem takes about 300K output tokens over the run's eight steps.
At 20 tokens a second on a 24 GB card that is about four hours. The published 9.8 ran on
Qwen3.8-27B in FP8 and BF16 through OpenRouter. A quantised copy on your card
is the same weights at lower precision. Readers in
[the HN thread](https://news.ycombinator.com/item?id=50320995)
([summary on zeli.app](https://zeli.app/story/50320995)) report Q5 and above
holding up where Q3 does not. Every step keeps its files, so a run that stops
resumes where it stopped.

Nothing leaves the machine. The runtime answers on localhost and Memdoor
keeps its state under `~/.memdoor`. The only outside address in a local setup
is GitHub, for the clone.
