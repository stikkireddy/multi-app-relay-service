import gradio as gr
import os
import jwt


def greet(request: gr.Request):
    headers = request.headers  # Access request headers
    headers_dict = {key: str(value) for key, value in headers.items()}
    token = headers_dict.get("x-forwarded-access-token", {})
    try:
        decoded = jwt.decode(token, options={"verify_signature": False})
    except Exception as e:
        decoded = {"error": str(e)}
    return decoded, headers_dict, dict(os.environ)

demo = gr.Interface(
    fn=greet,
    inputs=[],
    outputs=["json", "json", "json"],
)

port = os.environ.get("MULTI_APP_PORT", "8000")
demo.launch(server_port=int(port), root_path="/relay/app1", debug=True)
