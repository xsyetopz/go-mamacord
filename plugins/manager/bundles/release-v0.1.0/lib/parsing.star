HEX = {
    "0": 0,
    "1": 1,
    "2": 2,
    "3": 3,
    "4": 4,
    "5": 5,
    "6": 6,
    "7": 7,
    "8": 8,
    "9": 9,
    "a": 10,
    "b": 11,
    "c": 12,
    "d": 13,
    "e": 14,
    "f": 15,
}


def parse_hex(raw):
    if raw == None or raw == "":
        return None
    value = raw.lower()
    if value.startswith("#"):
        value = value[1:]
    if len(value) != 6:
        return "invalid"
    result = 0
    for index in range(6):
        digit = value[index]
        if digit not in HEX:
            return "invalid"
        result = result * 16 + HEX[digit]
    return result


def snowflake(raw):
    value = raw.strip()
    if "/" in value:
        value = value.split("/")[-1]
    if not value:
        return None
    for index in range(len(value)):
        if value[index] not in "0123456789":
            return None
    return value


def extension(file):
    filename = file["filename"].strip().lower()
    parts = filename.split(".")
    if len(parts) < 2:
        return ""
    return parts[-1]


def image_upload_error(file, prefix, max_bytes, allowed_extensions):
    if file == None:
        return [prefix + ".file_missing", {}]
    if file["size"] > max_bytes:
        return [prefix + ".file_too_large", {
            "Max": max_bytes,
            "Size": file["size"],
        }]
    file_extension = extension(file)
    if file_extension not in allowed_extensions:
        return [prefix + ".bad_extension", {"Ext": file_extension}]
    if file["width"] > 320 or file["height"] > 320:
        return [prefix + ".too_large_dims", {
            "Width": file["width"],
            "Height": file["height"],
        }]
    return None


def emoji_id(raw):
    value = raw.strip()
    if value.startswith("<") and value.endswith(">"):
        parts = value[:-1].split(":")
        if len(parts) >= 3:
            value = parts[-1]
    return snowflake(value)
