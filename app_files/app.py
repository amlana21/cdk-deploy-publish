import streamlit as st
from PyPDF2 import PdfReader
from langchain.text_splitter import CharacterTextSplitter
from langchain.embeddings.openai import OpenAIEmbeddings
from langchain.vectorstores import Pinecone
from langchain.chains.question_answering import load_qa_chain
from langchain.llms import OpenAI
from langchain.callbacks import get_openai_callback

import pinecone
import os
from dotenv import load_dotenv

def main():
    load_dotenv()
    st.set_page_config(page_title="Chat with your Doc", page_icon="📝")
    st.header("Ask your Question 📖")
    pdf_file = st.file_uploader("Upload the PDF", type="pdf")

    if pdf_file is not None:
        reader = PdfReader(pdf_file)
        text = ""
        for page in reader.pages:
            text += page.extract_text()

        # split into chunks
        text_splitter = CharacterTextSplitter(
            separator="\n",
            chunk_size=1000,
            chunk_overlap=200,
            length_function=len
        )
        chunks = text_splitter.split_text(text)

        # Init Pinecone index
        pinecone.init(
            api_key=os.getenv('PINECONE_API_KEY'),
            environment=''
        )
        index = pinecone.Index(os.getenv('PINECONE_INDEX_NAME'))

        embeddings = OpenAIEmbeddings(openai_api_key=os.getenv('OPENAI_API_KEY'))
        vector_store = Pinecone.from_texts(chunks, embeddings, index_name=os.getenv('PINECONE_INDEX_NAME'))
        # show user input
        user_question = st.text_input("Ask a question:")
        if user_question:
            llm = OpenAI()
            docs = vector_store.similarity_search(user_question)
            chain = load_qa_chain(llm, chain_type="stuff")
            with get_openai_callback() as cb:
                response = chain.run(input_documents=docs, question=user_question)
                print(cb)
            print(response)
            st.write(response)


if __name__ == '__main__':
    main()