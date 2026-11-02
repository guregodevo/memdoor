# Run Memdoor on an open-weight model, no key

A model on your machine is a provider. `memdoor connect local` finds it,
asks for no token. Nothing leaves the box. This page: empty machine to the
team-solve workflow on your GPU.

## 1. Run a model

Any OpenAI-compatible server. Four common ones:

| runtime | start it | listens on |
|---|---|---|
| [Ollama](https://ollama.com) | `ollama pull qwen3:8b` | 11434 |
| [llama.cpp](https://github.com/ggml-org/llama.cpp) | `llama-server -m model.gguf -c 32768` | 8080 |
| [LM Studio](https://lmstudio.ai) | load a model, start the server | 1234 |
| [vLLM](https://github.com/vllm-project/vllm) | `vllm serve Qwen/Qwen3.8-27B` | 8000 |

[janis](https://github.com/guregodevo/janis), our server for Apple silicon
and CUDA, listens on 8090 and also answers the decision model, so the token
savings run locally.

Model: one trained for tool calls, since a coder turn reads files, calls
tools and writes patches. Qwen3.8-27B is the one the published result ran
on. A 7B or 8B answers a turn on a laptop but loses the thread on long
work. Floor for team-solve: a 27B at Q5 or above, a 24 GB card or a 32 GB
Mac.

## 2. Give it a window

Ollama reads 4,096 tokens of prompt unless told otherwise. A coder turn's
prompt is longer; the model then answers a task it never saw and calls no
tool. Start Ollama with `OLLAMA_CONTEXT_LENGTH=32768` or make a tag:

```
printf 'FROM qwen3:8b\nPARAMETER num_ctx 32768\n' > Modelfile
ollama create qwen3:8b-32k -f Modelfile
```

llama.cpp: `-c 32768`. LM Studio and vLLM: the model's settings. 32K covers
a turn. Team-solve replies run past 100K, so give the 27B what the card
allows.

## 3. Connect

```
curl -fsSL https://memdoor.ai/install.sh | bash
memdoor connect local            # finds the runtime on this machine, probes it, keeps it
memdoor model                    # the model that answers, and what it lists for: nothing
```

Memdoor lists the model servers running on the gateway's machine (janis, Ollama, llama.cpp, LM Studio, vLLM, on their usual ports) and you pick one by number.
`--base http://localhost:8000/v1` names another. The probe reads the model
list and makes one call; a large model may take minutes to load. `/model
search` lists what the runtime serves, `/model qwen3:8b-32k` pins one. A
model pulled after the connect is found on its next pin.

Run in a terminal, `connect local` also keeps the tested model as your pin: new conversations start on `local:<model>` in place of an earlier pin, and `/model auto` goes back to the default.

On a box: `memdoor --gateway ssh://you@box connect local` installs Memdoor
there and connects the box's runtime. `@box:coder …` here runs the turn on
the box ([peers](/docs/providers#the-box-s-model-answers-the-laptop)).

## 4. A turn

```
cd your-project && memdoor tui
```

Ask it to read a file and fix something small. `memdoor meter` shows input
tokens per call; 4,096 on every call means the window of step 2 is not set.

## 5. The team on your GPU

The [team-solve workflow](/docs/team-solve) names no model. The harness
that scored 9.8 of 10 on LiveCodeBench Hard runs on whatever `connect
local` found:

```
git clone --depth 1 https://github.com/guregodevo/memdoor
mkdir -p ~/.memdoor/workflows && cp -r memdoor/examples/workflows/team-solve ~/.memdoor/workflows/
mkdir my-problem && cd my-problem && cp -r ../memdoor/examples/workflows/team-solve/sample/* .
memdoor workflow run team-solve --wait
```

Cost of a hard problem: about 300K output tokens over eight steps, about
four hours at 20 tokens a second on a 24 GB card. The published 9.8 ran on
Qwen3.8-27B in FP8 and BF16 through OpenRouter; a quantised copy is the same
weights at lower precision. Readers in
[the HN thread](https://news.ycombinator.com/item?id=50320995)
([summary on zeli.app](https://zeli.app/story/50320995)) report Q5 and above
holding up where Q3 does not. Each step keeps its files; a stopped run
resumes where it stopped.

Nothing leaves the machine: the runtime answers on localhost, state is under
`~/.memdoor`. The one outside address is GitHub, for the clone.
