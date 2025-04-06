from PyQt5.QtCore import (
    QFile,
    QIODevice,
    QByteArray,
    QDataStream,
    qUncompress
)
import json

class Annotation:
    def __init__(self, description, annotations):
        self.description = description
        self.annotations = annotations

def load_emoji_dictionary(dictfilename):
    dictfile = QFile(dictfilename)
    if not dictfile.open(QIODevice.ReadOnly):
        return []

    compressed = dictfile.readAll()
    dictfile.close()

    buf = qUncompress(compressed)
    stream = QDataStream(buf, QIODevice.ReadOnly)
    stream.setVersion(QDataStream.Qt_5_15)
    stream.setByteOrder(QDataStream.LittleEndian)

    filtered_emojis = []
    num_emojis = stream.readUInt32()
    
    for _ in range(num_emojis):
        # Read emoji (QByteArray)
        emoji_length = stream.readUInt32()
        emoji = bytes(stream.readRawData(emoji_length)).decode("utf-8")
        
        # Read description (QByteArray)
        desc_length = stream.readUInt32()
        description = bytes(stream.readRawData(desc_length)).decode("utf-8")
        
        # Read category
        category = stream.readInt32()
        
        # Read annotations list
        num_annotations = stream.readUInt32()
        annotations_list = []
        for __ in range(num_annotations):
            item_length = stream.readUInt32()
            item = bytes(stream.readRawData(item_length)).decode("utf-8")
            annotations_list.append(item)
        
        # Reconstruct the data structure
        annotation = Annotation(description, annotations_list)
        filtered_emojis.append((emoji, category, annotation))
    
    return filtered_emojis

emojis = load_emoji_dictionary("en.dict")
for emoji, category, annotation in emojis:
    emoji_list = []
    for emoji, category, annotation in emojis:
        emoji_entry = {
            "emoji": emoji,
            "category": category,
            "name": annotation.description
        }
        emoji_list.append(emoji_entry)
    
    # Write to JSON file
    with open("emojis.json", 'w', encoding='utf-8') as f:
        json.dump(emoji_list, f, ensure_ascii=False, indent=2)
